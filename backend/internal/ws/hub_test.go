package ws

import "testing"

func TestBroadcastReturnsFalseForInvalidPayload(t *testing.T) {
	hub := NewHub()

	if hub.Broadcast(Event{Type: "invalid", Data: func() {}}) {
		t.Fatal("expected invalid JSON payload to be rejected")
	}
	if stats := hub.Stats(); stats.DroppedBroadcasts != 1 {
		t.Fatalf("expected invalid payload to count as one dropped broadcast, got %d", stats.DroppedBroadcasts)
	}
}

func TestBroadcastDoesNotBlockWhenQueueIsFull(t *testing.T) {
	hub := NewHub()
	for i := 0; i < cap(hub.broadcast); i++ {
		if !hub.Broadcast(Event{Type: "test", Data: i}) {
			t.Fatalf("expected broadcast %d to be queued", i)
		}
	}

	if hub.Broadcast(Event{Type: "overflow", Data: "drop"}) {
		t.Fatal("expected full broadcast queue to reject new message")
	}
	stats := hub.Stats()
	if stats.DroppedBroadcasts != 1 {
		t.Fatalf("expected one dropped broadcast, got %d", stats.DroppedBroadcasts)
	}
}

func TestStatsReportsQueueAndClientCounters(t *testing.T) {
	hub := NewHub()

	if !hub.Broadcast(Event{Type: "test", Data: "queued"}) {
		t.Fatal("expected broadcast to be queued")
	}

	stats := hub.Stats()
	if stats.ActiveClients != 0 {
		t.Fatalf("expected no active clients, got %d", stats.ActiveClients)
	}
	if stats.BroadcastQueueDepth != 1 {
		t.Fatalf("expected queue depth 1, got %d", stats.BroadcastQueueDepth)
	}
	if stats.BroadcastQueueCapacity != cap(hub.broadcast) {
		t.Fatalf("expected capacity %d, got %d", cap(hub.broadcast), stats.BroadcastQueueCapacity)
	}
	if stats.DroppedBroadcasts != 0 {
		t.Fatalf("expected no dropped broadcasts, got %d", stats.DroppedBroadcasts)
	}
}

func TestRegisterAndUnregisterClientUpdatesActiveCount(t *testing.T) {
	hub := NewHub()
	client := &Client{hub: hub, send: make(chan []byte, 1)}

	hub.registerClient(client)
	hub.registerClient(client)

	stats := hub.Stats()
	if stats.ActiveClients != 1 {
		t.Fatalf("expected one active client after duplicate register, got %d", stats.ActiveClients)
	}

	hub.unregisterClient(client)
	stats = hub.Stats()
	if stats.ActiveClients != 0 {
		t.Fatalf("expected zero active clients after unregister, got %d", stats.ActiveClients)
	}

	if _, ok := <-client.send; ok {
		t.Fatal("expected client send channel to be closed after unregister")
	}

	hub.unregisterClient(client)
	stats = hub.Stats()
	if stats.ActiveClients != 0 {
		t.Fatalf("expected repeated unregister to keep zero active clients, got %d", stats.ActiveClients)
	}
}

func TestBroadcastMessageDropsSlowClientAndKeepsHealthyClient(t *testing.T) {
	hub := NewHub()
	slow := &Client{hub: hub, send: make(chan []byte, 1)}
	fast := &Client{hub: hub, send: make(chan []byte, 1)}
	hub.registerClient(slow)
	hub.registerClient(fast)

	slow.send <- []byte("already-full")
	payload := []byte(`{"type":"test","data":"queued"}`)
	hub.broadcastMessage(payload)

	stats := hub.Stats()
	if stats.ActiveClients != 1 {
		t.Fatalf("expected one remaining active client, got %d", stats.ActiveClients)
	}
	if stats.DroppedBroadcasts != 1 {
		t.Fatalf("expected one dropped broadcast for slow client, got %d", stats.DroppedBroadcasts)
	}
	if _, ok := hub.clients[slow]; ok {
		t.Fatal("expected slow client to be removed from hub")
	}
	if _, ok := hub.clients[fast]; !ok {
		t.Fatal("expected healthy client to remain registered")
	}

	select {
	case got := <-fast.send:
		if string(got) != string(payload) {
			t.Fatalf("expected fast client to receive payload %q, got %q", payload, got)
		}
	default:
		t.Fatal("expected fast client to receive broadcast payload")
	}
}
