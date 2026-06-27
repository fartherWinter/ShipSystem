FROM python:3.12-slim

WORKDIR /app
RUN useradd --create-home --shell /usr/sbin/nologin shipsystem
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY --chown=shipsystem:shipsystem app ./app
USER shipsystem
EXPOSE 8090
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]
