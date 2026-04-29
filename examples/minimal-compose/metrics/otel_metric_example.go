package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create OTLP metric exporter
	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint("localhost:4317"),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("Failed to create metric exporter: %v", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("otel-metric-example"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	// Create metric provider with periodic reader
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			exporter,
			sdkmetric.WithInterval(5*time.Second),
		)),
	)

	// Set the global meter provider
	otel.SetMeterProvider(provider)

	// Get a meter from the provider
	meter := provider.Meter("example-meter")

	// Create different types of metrics
	counter, err := meter.Int64Counter("example_counter", 
		metric.WithDescription("An example counter metric"),
		metric.WithUnit("requests"),
	)
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}

	gauge, err := meter.Float64Gauge("example_gauge",
		metric.WithDescription("An example gauge metric"),
		metric.WithUnit("percentage"),
	)
	if err != nil {
		log.Fatalf("Failed to create gauge: %v", err)
	}

	histogram, err := meter.Float64Histogram("example_histogram",
		metric.WithDescription("An example histogram metric"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		log.Fatalf("Failed to create histogram: %v", err)
	}

	fmt.Println("Starting to send metrics to OTEL collector at http://localhost:4317")
	fmt.Println("Press Ctrl+C to stop...")

	// Send metrics for 30 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 15; i++ {
		select {
		case <-ticker.C:
			// Increment counter with labels
			counter.Add(ctx, int64(rand.Intn(10)+1), metric.WithAttributes(
				attribute.String("service", "example"),
				attribute.String("environment", "development"),
				attribute.String("method", []string{"GET", "POST", "PUT"}[rand.Intn(3)]),
			))

			// Record gauge value
			gauge.Record(ctx, rand.Float64()*100, metric.WithAttributes(
				attribute.String("service", "example"),
				attribute.String("metric_type", "cpu_usage"),
			))

			// Record histogram value (simulating response times)
			histogram.Record(ctx, rand.Float64()*1000, metric.WithAttributes(
				attribute.String("service", "example"),
				attribute.String("endpoint", "/api/v1/users"),
			))

			fmt.Printf("Sent metrics batch %d/15\n", i+1)

		case <-ctx.Done():
			log.Println("Context cancelled, stopping...")
			return
		}
	}

	fmt.Println("Finished sending metrics. Shutting down...")

	// Shutdown the provider to flush remaining metrics
	if err := provider.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down provider: %v", err)
	}

	fmt.Println("Metrics sent successfully!")
}
