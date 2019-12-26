package tracing

import (
	"io"
	"sync"
	"time"

	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-lib/metrics"
	"google.golang.org/grpc/grpclog"

	opentracing "github.com/opentracing/opentracing-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

func InitTracing(name string, agentAddr string, logger jaeger.Logger) (closer io.Closer, err error) {
	tracer, closer, err := NewJaegerTracer(name, agentAddr, logger)
	if err != nil {
		return nil, err
	}
	opentracing.SetGlobalTracer(tracer)
	return closer, err
}

func NewJaegerTracer(serviceName string, agentAddr string, jLogger jaeger.Logger) (tracer opentracing.Tracer, closer io.Closer, err error) {
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  agentAddr,
		},
	}
	// frameworks.
	if jLogger == nil {
		jLogger = jaeger.StdLogger
	}
	jMetricsFactory := metrics.NullFactory

	// Initialize tracer with a logger and a metrics factory
	tracer, closer, err = cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory))

	// opentracing.SetGlobalTracer(tracer)
	if err != nil {
		grpclog.Errorf("Could not initialize jaeger tracer: %s", err.Error())
		return
	}
	return
}

type (
	SmStrSpan struct {
		sm sync.Map
	}
)

func (m *SmStrSpan) Load(key string) (value opentracing.Span, ok bool) {
	v, ok := m.sm.Load(key)
	if ok {
		rv, aok := v.(opentracing.Span)
		if !aok {
			panic(`type assert error`)
		}
		return rv, true
	}
	return nil, false
}

func (m *SmStrSpan) LoadOrDefault(key string, defval opentracing.Span) opentracing.Span {
	v, ok := m.Load(key)
	if !ok {
		return defval
	}
	return v
}

func (m *SmStrSpan) Store(key string, value opentracing.Span) {
	m.sm.Store(key, value)
}

func (m *SmStrSpan) LoadOrStore(key string, value opentracing.Span) (actual opentracing.Span, loaded bool) {
	a, l := m.sm.LoadOrStore(key, value)
	v, ok := a.(opentracing.Span)
	if !ok {
		panic("type assert error")
	}
	return v, l
}

func (m *SmStrSpan) Delete(key string) {
	m.sm.Delete(key)
}

func (m *SmStrSpan) Range(f func(key string, value opentracing.Span) bool) {
	m.sm.Range(func(k, v interface{}) bool {
		rk, ok := k.(string)
		if !ok {
			panic("type assert error")
		}
		rv, ok := v.(opentracing.Span)
		if !ok {
			panic("type assert error")
		}

		return f(rk, rv)
	})
}
