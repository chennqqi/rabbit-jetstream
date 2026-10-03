package jetstream

import (
	"context"
	"errors"
	"testing"

	jsapi "github.com/nats-io/nats.go/jetstream"
)

// All other SDK methods are nil: enumeration or mutation would panic.
type detailJS struct {
	jsapi.JetStream
	ctx                  context.Context
	stream, name         string
	err, pushErr         error
	pullCalls, pushCalls int
}

func (f *detailJS) check(ctx context.Context, stream, name string) {
	if ctx != f.ctx || stream != f.stream || name != f.name {
		panic("exact identity or request context lost")
	}
}

func (f *detailJS) Consumer(ctx context.Context, stream, name string) (jsapi.Consumer, error) {
	f.check(ctx, stream, name)
	f.pullCalls++
	if f.err != nil {
		return nil, f.err
	}
	return &fakeConsumer{info: &jsapi.ConsumerInfo{Stream: stream, Name: name, NumPending: 42}}, nil
}

type detailPush struct {
	jsapi.PushConsumer
	info *jsapi.ConsumerInfo
}

func (p *detailPush) CachedInfo() *jsapi.ConsumerInfo { return p.info }

func (f *detailJS) PushConsumer(ctx context.Context, stream, name string) (jsapi.PushConsumer, error) {
	f.check(ctx, stream, name)
	f.pushCalls++
	if f.pushErr != nil {
		return nil, f.pushErr
	}
	return &detailPush{info: &jsapi.ConsumerInfo{Stream: stream, Name: name, NumPending: 42, Config: jsapi.ConsumerConfig{DeliverSubject: "delivery"}}}, nil
}

func TestConsumerExactRead(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		err, pushErr, wantErr error
		mode                  string
		pushCalls             int
	}{
		{name: "pull", mode: "pull"},
		{name: "push", err: jsapi.ErrNotPullConsumer, mode: "push", pushCalls: 1},
		{name: "missing stream", err: jsapi.ErrStreamNotFound, wantErr: ErrNotFound},
		{name: "missing consumer", err: jsapi.ErrConsumerNotFound, wantErr: ErrNotFound},
		{name: "timeout", err: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
		{name: "canceled", err: context.Canceled, wantErr: context.Canceled},
		{name: "push disappeared", err: jsapi.ErrNotPullConsumer, pushErr: jsapi.ErrConsumerNotFound, wantErr: ErrNotFound, pushCalls: 1},
		{name: "push stream disappeared", err: jsapi.ErrNotPullConsumer, pushErr: jsapi.ErrStreamNotFound, wantErr: ErrNotFound, pushCalls: 1},
		{name: "push timeout", err: jsapi.ErrNotPullConsumer, pushErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded, pushCalls: 1},
		{name: "type changed", err: jsapi.ErrNotPullConsumer, pushErr: jsapi.ErrNotPushConsumer, wantErr: jsapi.ErrNotPushConsumer, pushCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backend := &detailJS{ctx: ctx, stream: "orders", name: "worker", err: tc.err, pushErr: tc.pushErr}
			client := &Client{js: backend}
			got, err := client.Consumer(ctx, "orders", "worker")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if err != nil && got != nil {
				t.Fatalf("failure returned fabricated observation: %+v", got)
			}
			if err == nil && (got.Stream != "orders" || got.Name != "worker" || got.Mode != tc.mode || got.Pending != 42) {
				t.Fatalf("observation = %+v", got)
			}
			if backend.pullCalls != 1 || backend.pushCalls != tc.pushCalls {
				t.Fatalf("calls = %d/%d", backend.pullCalls, backend.pushCalls)
			}
		})
	}
}
