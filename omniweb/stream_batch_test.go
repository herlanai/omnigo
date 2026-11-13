package omniweb

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStreamBatchAcknowledgmentsReleaseWholePrefix(t *testing.T) {
	flow := newStreamFlow("")
	var asks []uint64
	writer := &streamWriter{flow: flow, ctx: t.Context(), gate: make(chan struct{}, 1)}
	writer.write = func(frame streamFrame) error {
		if frame.Ask {
			asks = append(asks, frame.Seq)
			if err := flow.confirm(frame.Seq); err != nil {
				return err
			}
			if flow.bytes != 0 || len(flow.pending) != 0 {
				t.Fatal("batch did not release prefix")
			}
		}
		return nil
	}
	for i := 0; i < 10; i++ {
		if err := writer.send(strings.Repeat("x", 200<<10)); err != nil {
			t.Fatal(err)
		}
	}
	if len(asks) != 3 || asks[0] != 3 || asks[1] != 6 || asks[2] != 9 {
		t.Fatal("wrong half-window requests", asks)
	}
	if flow.sent != 10 || flow.ack != 9 || len(flow.pending) != 1 {
		t.Fatal("wrong outstanding tail", flow.sent, flow.ack)
	}
}

func TestStreamBatchWaitsWithoutConsumptionAndCancels(t *testing.T) {
	flow := newStreamFlow("")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var values int
	writer := &streamWriter{flow: flow, ctx: ctx, gate: make(chan struct{}, 1), write: func(frame streamFrame) error {
		values++
		if frame.Ask != (frame.Seq == 3) {
			t.Errorf("unexpected marker at %d", frame.Seq)
		}
		return nil
	}}
	for i := 0; i < 4; i++ {
		if err := writer.send(strings.Repeat("x", 220<<10)); err != nil {
			t.Fatal(err)
		}
	}
	// Four values occupy > 3/4 of the window. The next one cannot fit until ACK.
	if flow.bytes+(220<<10)+258 <= streamWindow {
		t.Fatal("test did not fill window")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- writer.send(strings.Repeat("x", 220<<10)) }()
	cancel()
	if err := <-stopped; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if values != 4 || flow.sent != 4 {
		t.Fatal("unconfirmed window was exceeded")
	}
}

func TestSendContextCancelsWithoutCancelingWholeCall(t *testing.T) {
	flow := newStreamFlow("")
	if _, err := flow.reserve(t.Context(), streamWindow); err != nil {
		t.Fatal(err)
	}
	writer := &streamWriter{flow: flow, ctx: t.Context(), gate: make(chan struct{}, 1), write: func(streamFrame) error { return nil }}
	ch := &Channel[string]{writer: writer}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- ch.SendContext(ctx, "blocked") }()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if writer.ctx.Err() != nil {
		t.Fatal("per-send cancellation ended method")
	}
	if err := flow.confirm(1); err != nil {
		t.Fatal(err)
	}
	if err := ch.Send("next"); err != nil {
		t.Fatal("send gate leaked", err)
	}
}
