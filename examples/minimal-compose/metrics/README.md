# OpenTelemetry Metrics Example

This example demonstrates how to create and export various types of metrics using OpenTelemetry in Go.

## Features

The example showcases:
- **Counter metrics**: Incrementing values (e.g., request counts)
- **Gauge metrics**: Point-in-time measurements (e.g., CPU usage percentage)
- **Histogram metrics**: Distribution measurements (e.g., response times)

All metrics are exported to an OpenTelemetry collector via gRPC.

## Prerequisites

Before running the example, you need an OpenTelemetry collector running on `localhost:4317`. 

You can start one using Docker:
```bash
# Run the OpenTelemetry collector
docker run -p 4317:4317 -p 4318:4318 \
    otel/opentelemetry-collector-contrib:latest \
    --config-file=/etc/otel-collector-config.yaml
```

## Running the Example

1. Navigate to the metrics example directory:
   ```bash
   cd examples/otel/metrics
   ```

2. Build the example:
   ```bash
   go build -o otel_metric_example otel_metric_example.go
   ```

3. Run the example:
   ```bash
   ./otel_metric_example
   ```

The example will:
- Connect to the OpenTelemetry collector at `localhost:4317`
- Send various metric types with labels/attributes
- Run for 30 seconds (15 batches every 2 seconds)
- Display progress messages
- Gracefully shutdown

## Metrics Generated

### Counter: `example_counter`
- **Description**: An example counter metric
- **Unit**: requests
- **Attributes**: `service`, `environment`, `method`

### Gauge: `example_gauge`
- **Description**: An example gauge metric  
- **Unit**: percentage
- **Attributes**: `service`, `metric_type`

### Histogram: `example_histogram`
- **Description**: An example histogram metric
- **Unit**: ms
- **Attributes**: `service`, `endpoint`

## Configuration

The example is configured to:
- Export metrics every 5 seconds via periodic reader
- Send metrics to `localhost:4317` (insecure gRPC)
- Use service name: `otel-metric-example`
- Use service version: `1.0.0`

You can modify these settings in the `otel_metric_example.go` file as needed.
