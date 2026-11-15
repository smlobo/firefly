// Copyright © 2026 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/hyperledger-firefly/common/pkg/fftypes"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/hyperledger-firefly/firefly"

var tracerProvider *sdktrace.TracerProvider

// Init configures OTLP/HTTP exporting. The exporter reads
// OTEL_EXPORTER_OTLP_ENDPOINT and defaults to http://localhost:4318.
func Init(ctx context.Context) error {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return err
	}

	resourceAttrs := []attribute.KeyValue{
		attribute.String("service.name", "firefly"),
		attribute.String("service.instance.id", fftypes.NewUUID().String()),
		attribute.String("host.arch", runtime.GOARCH),
	}
	if len(os.Args) > 0 {
		resourceAttrs = append(resourceAttrs, attribute.String("process.command", os.Args[0]))
	}
	if osVersion := operatingSystemVersion(ctx); osVersion != "" {
		resourceAttrs = append(resourceAttrs, attribute.String("os.version", osVersion))
	}

	res, resourceErr := resource.New(ctx,
		resource.WithHost(),
		resource.WithHostID(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(resourceAttrs...),
	)
	if resourceErr != nil {
		// Resource detection is best-effort; a missing host attribute should not
		// prevent FireFly from starting or exporting traces.
		otel.Handle(resourceErr)
	}

	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return nil
}

// operatingSystemVersion returns the kernel release, matching the value
// reported by Node's os.version() and the OpenTelemetry os.version attribute.
// It is best-effort because uname is not available on every platform.
func operatingSystemVersion(ctx context.Context) string {
	version, err := exec.CommandContext(ctx, "uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(version))
}

// Shutdown flushes pending spans and closes the OTLP exporter.
func Shutdown(ctx context.Context) error {
	if tracerProvider == nil {
		return nil
	}
	return tracerProvider.Shutdown(ctx)
}

// StartServerSpan extracts the W3C trace context from an incoming HTTP request
// and starts a server span beneath the caller's span.
func StartServerSpan(ctx context.Context, headers http.Header, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
	return otel.Tracer(instrumentationName).Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

// StartSpan starts a child span for work performed within a request.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(instrumentationName).Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartRootSpan starts a new trace, detached from any request or connection
// context. This is useful for asynchronous work that should not extend the
// lifetime of the request that originally initiated it.
func StartRootSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(instrumentationName).Start(ctx, name,
		trace.WithNewRoot(),
		trace.WithAttributes(attrs...),
	)
}

// StartClientSpan starts a client span for an outbound dependency request.
func StartClientSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(instrumentationName).Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

// InjectTraceContext writes the active W3C trace context into an outbound
// request carrier.
func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// TraceParent returns the W3C traceparent for the active span.
func TraceParent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	InjectTraceContext(ctx, carrier)
	return carrier.Get("traceparent")
}

// EndSpan records an error, when present, and ends the span.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
