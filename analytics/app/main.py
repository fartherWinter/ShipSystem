import asyncio
import logging
import math
import os
import secrets
from datetime import datetime, timezone
from dataclasses import dataclass
from typing import Any

import httpx
from fastapi import FastAPI, Header, HTTPException, Request, status
from fastapi.middleware.cors import CORSMiddleware
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, field_validator

from app.battle_sim import BattleSimulation


logger = logging.getLogger("shipsystem.analytics")

APP_ENV = os.getenv("APP_ENV", "development").lower()
GO_API_BASE = os.getenv("GO_API_BASE", "http://localhost:8080/api/v1")
GO_API_TOKEN = os.getenv("GO_API_TOKEN", "")
ANALYTICS_ADMIN_TOKEN = os.getenv("ANALYTICS_ADMIN_TOKEN", "")
SIM_INTERVAL_SECONDS = float(os.getenv("SIM_INTERVAL_SECONDS", "3"))
BATTLE_INTERVAL_SECONDS = float(os.getenv("BATTLE_INTERVAL_SECONDS", "1.2"))
HTTP_RETRY_ATTEMPTS = int(os.getenv("HTTP_RETRY_ATTEMPTS", "3"))
HTTP_TIMEOUT_SECONDS = float(os.getenv("HTTP_TIMEOUT_SECONDS", "10"))
HTTP_RETRY_BACKOFF_SECONDS = float(os.getenv("HTTP_RETRY_BACKOFF_SECONDS", "0.2"))
HTTP_RETRY_BACKOFF_MAX_SECONDS = float(os.getenv("HTTP_RETRY_BACKOFF_MAX_SECONDS", "2"))
SUPPORTED_BATTLE_SCENARIOS = {"open-water-duel", "close-quarter-barrage"}
MAX_ERROR_SUMMARY_LENGTH = 300
REQUEST_ID_HEADER = "X-Request-ID"
REQUEST_ID_STATE_KEY = "request_id"


def require_finite(value: float, field_name: str) -> float:
    if not math.isfinite(value):
        raise ValueError(f"{field_name} must be finite")
    return value


def validate_longitude(value: float) -> float:
    require_finite(value, "longitude")
    if value < -180 or value > 180:
        raise ValueError("longitude must be between -180 and 180")
    return value


def validate_latitude(value: float) -> float:
    require_finite(value, "latitude")
    if value < -90 or value > 90:
        raise ValueError("latitude must be between -90 and 90")
    return value


def validate_speed_knots(value: float) -> float:
    require_finite(value, "speedKnots")
    if value < 0:
        raise ValueError("speedKnots must be greater than or equal to 0")
    return value


def validate_course(value: float) -> float:
    require_finite(value, "course")
    if value < 0 or value >= 360:
        raise ValueError("course must be between 0 inclusive and 360 exclusive")
    return value


def validate_session_id(value: str) -> str:
    value = value.strip()
    if not value:
        raise ValueError("sessionId is required")
    return value


def validate_non_negative(value: float, field_name: str) -> float:
    require_finite(value, field_name)
    if value < 0:
        raise ValueError(f"{field_name} must be greater than or equal to 0")
    return value


def validate_runtime_config(app_env: str, analytics_admin_token: str, go_api_token: str) -> None:
    validate_non_negative(HTTP_TIMEOUT_SECONDS, "HTTP_TIMEOUT_SECONDS")
    validate_non_negative(HTTP_RETRY_BACKOFF_SECONDS, "HTTP_RETRY_BACKOFF_SECONDS")
    validate_non_negative(HTTP_RETRY_BACKOFF_MAX_SECONDS, "HTTP_RETRY_BACKOFF_MAX_SECONDS")
    if app_env not in {"production", "prod"}:
        return
    if len(analytics_admin_token) < 24:
        raise RuntimeError("ANALYTICS_ADMIN_TOKEN must be set to a strong value in production")
    if len(go_api_token) < 24:
        raise RuntimeError("GO_API_TOKEN must be set to a strong value in production")


validate_runtime_config(APP_ENV, ANALYTICS_ADMIN_TOKEN, GO_API_TOKEN)

app = FastAPI(title="ShipSystem Analytics", version="0.1.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=os.getenv("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000").split(","),
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.middleware("http")
async def request_id_middleware(request: Request, call_next):
    request_id = normalize_request_id(request.headers.get(REQUEST_ID_HEADER)) or new_request_id()
    setattr(request.state, REQUEST_ID_STATE_KEY, request_id)
    response = await call_next(request)
    response.headers[REQUEST_ID_HEADER] = request_id
    return response


@app.exception_handler(HTTPException)
async def http_exception_handler(request: Request, exc: HTTPException) -> JSONResponse:
    return error_response(request, exc.status_code, str(exc.detail), detail=exc.detail, headers=exc.headers)


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError) -> JSONResponse:
    return error_response(
        request,
        status.HTTP_422_UNPROCESSABLE_CONTENT,
        "request validation failed",
        detail=json_safe_validation_errors(exc),
    )


@app.exception_handler(Exception)
async def unhandled_exception_handler(request: Request, exc: Exception) -> JSONResponse:
    logger.exception(
        "unhandled analytics exception request_id=%s method=%s path=%s",
        current_request_id(request),
        request.method,
        request.url.path,
    )
    return error_response(request, status.HTTP_500_INTERNAL_SERVER_ERROR, "internal server error")


def error_response(
    request: Request,
    status_code: int,
    message: str,
    *,
    detail: Any | None = None,
    headers: dict[str, str] | None = None,
) -> JSONResponse:
    request_id = current_request_id(request) or new_request_id()
    content: dict[str, Any] = {"message": message, "requestId": request_id}
    if detail is not None:
        content["detail"] = detail
    response_headers = dict(headers or {})
    response_headers[REQUEST_ID_HEADER] = request_id
    return JSONResponse(status_code=status_code, content=content, headers=response_headers)


def json_safe_validation_errors(exc: RequestValidationError) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []
    for item in exc.errors():
        safe_item: dict[str, Any] = {}
        for key, value in item.items():
            if key == "ctx":
                safe_item[key] = {ctx_key: str(ctx_value) for ctx_key, ctx_value in value.items()}
            elif key == "loc":
                safe_item[key] = list(value)
            else:
                safe_item[key] = value
        items.append(safe_item)
    return items


_simulator_task: asyncio.Task | None = None
_battle_tasks: dict[str, asyncio.Task] = {}


@dataclass
class DeliveryResult:
    success: bool
    attempts: int
    status_code: int | None = None
    error: str = ""


class DeliveryMetrics:
    def __init__(self) -> None:
        self.success_count = 0
        self.failure_count = 0
        self.retry_count = 0
        self.dropped_count = 0
        self.last_success_at: datetime | None = None
        self.last_failure_at: datetime | None = None
        self.last_error = ""

    def record(self, result: DeliveryResult) -> None:
        if result.attempts > 1:
            self.retry_count += result.attempts - 1
        if result.success:
            self.success_count += 1
            self.last_success_at = datetime.now(timezone.utc)
            return
        self.failure_count += 1
        self.dropped_count += 1
        self.last_failure_at = datetime.now(timezone.utc)
        self.last_error = result.error

    def to_payload(self) -> dict[str, Any]:
        return {
            "successCount": self.success_count,
            "failureCount": self.failure_count,
            "retryCount": self.retry_count,
            "droppedCount": self.dropped_count,
            "lastSuccessAt": self.last_success_at.isoformat() if self.last_success_at else None,
            "lastFailureAt": self.last_failure_at.isoformat() if self.last_failure_at else None,
            "lastError": self.last_error,
        }


delivery_metrics = DeliveryMetrics()


class LocationInput(BaseModel):
    ship_id: int = Field(alias="shipId")
    longitude: float
    latitude: float
    speed_knots: float = Field(default=0, alias="speedKnots")
    course: float = 0
    reported_at: datetime | None = Field(default=None, alias="reportedAt")

    model_config = {"populate_by_name": True}

    @field_validator("ship_id")
    @classmethod
    def ship_id_must_be_positive(cls, value: int) -> int:
        if value <= 0:
            raise ValueError("shipId must be positive")
        return value

    @field_validator("longitude")
    @classmethod
    def longitude_must_be_valid(cls, value: float) -> float:
        return validate_longitude(value)

    @field_validator("latitude")
    @classmethod
    def latitude_must_be_valid(cls, value: float) -> float:
        return validate_latitude(value)

    @field_validator("speed_knots")
    @classmethod
    def speed_knots_must_be_valid(cls, value: float) -> float:
        return validate_speed_knots(value)

    @field_validator("course")
    @classmethod
    def course_must_be_valid(cls, value: float) -> float:
        return validate_course(value)


class SimulationStartRequest(BaseModel):
    ship_ids: list[int] = Field(default=[1, 2], alias="shipIds")
    origin_longitude: float = Field(default=121.49, alias="originLongitude")
    origin_latitude: float = Field(default=31.23, alias="originLatitude")

    model_config = {"populate_by_name": True}

    @field_validator("ship_ids")
    @classmethod
    def ship_ids_must_be_positive(cls, value: list[int]) -> list[int]:
        if not value:
            raise ValueError("shipIds must not be empty")
        if any(ship_id <= 0 for ship_id in value):
            raise ValueError("shipIds must contain only positive IDs")
        return value

    @field_validator("origin_longitude")
    @classmethod
    def origin_longitude_must_be_valid(cls, value: float) -> float:
        return validate_longitude(value)

    @field_validator("origin_latitude")
    @classmethod
    def origin_latitude_must_be_valid(cls, value: float) -> float:
        return validate_latitude(value)


class BattleSimulationStartRequest(BaseModel):
    session_id: str = Field(alias="sessionId")
    scenario_code: str = Field(default="open-water-duel", alias="scenarioCode")
    origin_longitude: float = Field(default=121.49, alias="originLongitude")
    origin_latitude: float = Field(default=31.23, alias="originLatitude")
    seed: int = 7

    model_config = {"populate_by_name": True}

    @field_validator("session_id")
    @classmethod
    def session_id_must_be_valid(cls, value: str) -> str:
        return validate_session_id(value)

    @field_validator("scenario_code")
    @classmethod
    def scenario_code_must_be_supported(cls, value: str) -> str:
        value = value.strip()
        if not value:
            return "open-water-duel"
        if value not in SUPPORTED_BATTLE_SCENARIOS:
            raise ValueError("scenarioCode is not supported")
        return value

    @field_validator("origin_longitude")
    @classmethod
    def battle_origin_longitude_must_be_valid(cls, value: float) -> float:
        return validate_longitude(value)

    @field_validator("origin_latitude")
    @classmethod
    def battle_origin_latitude_must_be_valid(cls, value: float) -> float:
        return validate_latitude(value)


class BattleSimulationStopRequest(BaseModel):
    session_id: str = Field(alias="sessionId")

    model_config = {"populate_by_name": True}

    @field_validator("session_id")
    @classmethod
    def session_id_must_be_valid(cls, value: str) -> str:
        return validate_session_id(value)


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/simulate/status")
async def simulation_status(
    authorization: str | None = Header(default=None),
    x_analytics_token: str | None = Header(default=None),
) -> dict[str, Any]:
    require_admin_token(authorization, x_analytics_token)
    return {
        "running": task_running(_simulator_task),
        "battleSessions": {
            session_id: {"running": task_running(task), "done": task.done()}
            for session_id, task in _battle_tasks.items()
        },
        "delivery": delivery_metrics.to_payload(),
    }


@app.post("/analyze/location")
async def analyze_location(payload: LocationInput) -> dict[str, Any]:
    alarms: list[dict[str, Any]] = []
    if payload.speed_knots > 18:
        alarms.append(
            {
                "type": "OVERSPEED",
                "level": "WARN",
                "message": f"当前航速 {payload.speed_knots:.1f} 节，超过 18 节阈值",
            }
        )
    if not in_demo_fence(payload.longitude, payload.latitude):
        alarms.append(
            {
                "type": "OUT_OF_AREA",
                "level": "WARN",
                "message": "船舶驶出演示监管区域",
            }
        )
    return {"alarms": alarms}


@app.post("/simulate/start")
async def start_simulation(
    req: SimulationStartRequest,
    request: Request,
    authorization: str | None = Header(default=None),
    x_analytics_token: str | None = Header(default=None),
) -> dict[str, Any]:
    require_admin_token(authorization, x_analytics_token)
    global _simulator_task
    if _simulator_task and not _simulator_task.done():
        return {"running": True, "message": "模拟器已在运行"}
    _simulator_task = asyncio.create_task(run_simulator(req, authorization, current_request_id(request)))
    return {"running": True, "shipIds": req.ship_ids}


@app.post("/simulate/stop")
async def stop_simulation(
    authorization: str | None = Header(default=None),
    x_analytics_token: str | None = Header(default=None),
) -> dict[str, bool]:
    require_admin_token(authorization, x_analytics_token)
    global _simulator_task
    if _simulator_task and not _simulator_task.done():
        _simulator_task.cancel()
        try:
            await _simulator_task
        except asyncio.CancelledError:
            pass
    _simulator_task = None
    return {"running": False}


@app.post("/simulate/battle/start")
async def start_battle_simulation(
    req: BattleSimulationStartRequest,
    request: Request,
    authorization: str | None = Header(default=None),
    x_analytics_token: str | None = Header(default=None),
) -> dict[str, Any]:
    require_admin_token(authorization, x_analytics_token)
    task = _battle_tasks.get(req.session_id)
    if task and not task.done():
        return {"running": True, "sessionId": req.session_id}
    simulation = BattleSimulation(
        session_id=req.session_id,
        scenario_code=req.scenario_code,
        origin_longitude=req.origin_longitude,
        origin_latitude=req.origin_latitude,
        seed=req.seed,
    )
    _battle_tasks[req.session_id] = asyncio.create_task(run_battle_simulator(simulation, authorization, current_request_id(request)))
    return {"running": True, "sessionId": req.session_id, "scenarioCode": req.scenario_code}


@app.post("/simulate/battle/stop")
async def stop_battle_simulation(
    req: BattleSimulationStopRequest,
    authorization: str | None = Header(default=None),
    x_analytics_token: str | None = Header(default=None),
) -> dict[str, Any]:
    require_admin_token(authorization, x_analytics_token)
    task = _battle_tasks.get(req.session_id)
    if task and not task.done():
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass
    _battle_tasks.pop(req.session_id, None)
    return {"running": False, "sessionId": req.session_id}


async def run_simulator(req: SimulationStartRequest, authorization: str | None, request_id: str = "") -> None:
    headers = callback_headers(authorization, request_id)
    step = 0
    async with callback_http_client() as client:
        while True:
            for index, ship_id in enumerate(req.ship_ids):
                angle = (step + index * 60) / 24
                longitude = req.origin_longitude + math.cos(angle) * 0.08
                latitude = req.origin_latitude + math.sin(angle) * 0.05
                speed = 10 + abs(math.sin(angle)) * 12
                payload = {
                    "longitude": round(longitude, 6),
                    "latitude": round(latitude, 6),
                    "speedKnots": round(speed, 2),
                    "course": round((angle * 180 / math.pi) % 360, 2),
                    "reportedAt": datetime.now(timezone.utc).isoformat(),
                }
                result = await post_with_retry(
                    client,
                    f"{GO_API_BASE}/ships/{ship_id}/locations",
                    json=payload,
                    headers=headers,
                    context=f"ship location ship_id={ship_id}",
                )
                delivery_metrics.record(result)
            step += 1
            await asyncio.sleep(SIM_INTERVAL_SECONDS)


async def run_battle_simulator(simulation: BattleSimulation, authorization: str | None, request_id: str = "") -> None:
    headers = callback_headers(authorization, request_id)
    async with callback_http_client() as client:
        while True:
            payload = simulation.step(BATTLE_INTERVAL_SECONDS)
            result = await post_with_retry(
                client,
                f"{GO_API_BASE}/radar/reports",
                json=payload,
                headers=headers,
                context=f"battle radar session_id={simulation.session_id}",
            )
            delivery_metrics.record(result)
            if simulation.status != "running":
                break
            await asyncio.sleep(BATTLE_INTERVAL_SECONDS)


def in_demo_fence(longitude: float, latitude: float) -> bool:
    return 121.25 <= longitude <= 121.75 and 31.0 <= latitude <= 31.45


def require_admin_token(authorization: str | None, x_analytics_token: str | None) -> None:
    if not ANALYTICS_ADMIN_TOKEN:
        return
    if x_analytics_token == ANALYTICS_ADMIN_TOKEN:
        return
    if authorization == f"Bearer {ANALYTICS_ADMIN_TOKEN}":
        return
    raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="analytics admin token is required")


def task_running(task: asyncio.Task | None) -> bool:
    return bool(task and not task.done())


def callback_headers(authorization: str | None, request_id: str = "") -> dict[str, str]:
    forward_auth = f"Bearer {GO_API_TOKEN}" if GO_API_TOKEN else authorization
    headers = {"Authorization": forward_auth} if forward_auth else {}
    if request_id:
        headers[REQUEST_ID_HEADER] = request_id
    return headers


def normalize_request_id(value: str | None) -> str:
    value = (value or "").strip()
    if not value or len(value) > 128:
        return ""
    if any(ord(ch) < 33 or ord(ch) > 126 for ch in value):
        return ""
    return value


def new_request_id() -> str:
    return secrets.token_hex(16)


def current_request_id(request: Request) -> str:
    value = getattr(request.state, REQUEST_ID_STATE_KEY, "")
    return value if isinstance(value, str) else ""


def callback_http_client() -> httpx.AsyncClient:
    return httpx.AsyncClient(timeout=HTTP_TIMEOUT_SECONDS)


def retry_backoff_seconds(attempt: int) -> float:
    return min(HTTP_RETRY_BACKOFF_MAX_SECONDS, HTTP_RETRY_BACKOFF_SECONDS * attempt)


async def post_with_retry(
    client: httpx.AsyncClient,
    url: str,
    *,
    json: dict[str, Any],
    headers: dict[str, str],
    context: str,
) -> DeliveryResult:
    max_attempts = max(HTTP_RETRY_ATTEMPTS, 1)
    last_error = ""
    last_status_code: int | None = None
    for attempt in range(1, max_attempts + 1):
        try:
            response = await client.post(url, json=json, headers=headers)
            last_status_code = response.status_code
            if 200 <= response.status_code < 300:
                return DeliveryResult(success=True, attempts=attempt, status_code=response.status_code)
            last_error = response_error_summary(response)
            if not retryable_status(response.status_code):
                logger.warning(
                    "dropping non-retryable post %s attempt=%s status=%s request_id=%s error=%s",
                    context,
                    attempt,
                    response.status_code,
                    headers.get(REQUEST_ID_HEADER, ""),
                    last_error,
                )
                return DeliveryResult(False, attempt, response.status_code, last_error)
            logger.warning(
                "failed to post %s attempt=%s status=%s request_id=%s error=%s",
                context,
                attempt,
                response.status_code,
                headers.get(REQUEST_ID_HEADER, ""),
                last_error,
            )
            if attempt >= max_attempts:
                break
            await asyncio.sleep(retry_backoff_seconds(attempt))
        except httpx.HTTPError as exc:
            last_error = str(exc)
            logger.warning(
                "failed to post %s attempt=%s request_id=%s error=%s",
                context,
                attempt,
                headers.get(REQUEST_ID_HEADER, ""),
                exc,
            )
            if attempt >= max_attempts:
                break
            await asyncio.sleep(retry_backoff_seconds(attempt))
    return DeliveryResult(False, max_attempts, last_status_code, last_error)


def retryable_status(status_code: int) -> bool:
    return status_code == 429 or status_code >= 500


def response_error_summary(response: httpx.Response) -> str:
    text = response.text.strip()
    if len(text) > MAX_ERROR_SUMMARY_LENGTH:
        text = text[:MAX_ERROR_SUMMARY_LENGTH] + "..."
    if text:
        return text
    return f"HTTP {response.status_code}"
