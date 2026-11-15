---
Distributed Tracing
---

FireFly exports OpenTelemetry traces over OTLP/HTTP. The exporter reads
`OTEL_EXPORTER_OTLP_ENDPOINT`, which defaults to `http://localhost:4318`.

## API request traces

FireFly starts a server span for each JSON and form-upload API request, including requests that fail during
orchestration or authorization. The span records the HTTP method and API route. Successful responses include the
response status, and responses containing an operation include its FireFly operation ID. Handler errors are recorded
on the span.

FireFly extracts the W3C `traceparent` header from incoming HTTP requests. The resulting server span continues the
caller’s trace, and child spans inherit that context.

## EVMConnect submissions

For Ethereum transaction submissions, FireFly creates an EVMConnect client span beneath the API request span. It injects
the span context as a `traceparent` HTTP header on the EVMConnect request, allowing EVMConnect instrumentation to join
the same trace.

## Asynchronous WebSocket events

Each event FireFly sends over WebSockets gets a new root trace, independent of the API request that originally submitted
the transaction. FireFly adds the event span’s W3C context as a top-level `traceparent` property in the event payload.
In a batched message, each event has its own `traceparent`.

A WebSocket receiver can extract the property and use the resulting context as the parent for its consumer span. For
example, in TypeScript:

```ts
const parentContext = propagation.extract(ROOT_CONTEXT, {
  traceparent: event.traceparent,
});
const span = tracer.startSpan("firefly.event.process", {
  kind: SpanKind.CONSUMER,
}, parentContext);
```

## Resource metadata

FireFly attaches resource attributes to its spans, including `service.name`, a generated `service.instance.id`, host
name, ID and architecture, OS type, description and best-effort version, and process ID, command, command arguments,
executable, owner and Go runtime details. It also reports the OpenTelemetry SDK name, language and version. Resource
detection is best-effort; an unavailable host attribute does not prevent FireFly from starting.

`process.command_args` contains the process arguments and may include values passed on the command line. Avoid passing
secrets as command-line arguments if those values should not be exported with telemetry.

## OTLP receiver configuration

When FireFly runs in Docker and the receiver is on the host, configure the FireFly container with:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4318
```

When FireFly and the receiver share a Docker network, use the receiver’s service name instead, for example:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
```
