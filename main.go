package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

func initTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// (a) トレースを送信するクライアントの初期化
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint("otlp-vaxila.mackerelio.com"),
		otlptracehttp.WithHeaders(map[string]string{
			"Accept":           "*/*",
			"Mackerel-Api-Key": os.Getenv("MACKEREL_APIKEY"),
		}),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	)
	// (b) トレースエクスポーターの初期化
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, err
	}

	// (c) リソース情報の設定
	resources, err := resource.New(
		ctx,
		resource.WithProcessPID(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName("sample-service"),
			semconv.DeploymentEnvironment("development"),
			semconv.ServiceNamespace("monitoring"),
		),
	)
	if err != nil {
		return nil, err
	}

	// (d) トレーサープロバイダーの初期化
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
	)
	otel.SetTracerProvider(tp)

	return tp, nil
}

// 引数で与えられた秒数だけスリープする関数
func superHeavyFunc(ctx context.Context, t trace.Tracer, n int) {
	// (B) superHeavyFunc のスパンを作成
	_, span := t.Start(ctx, fmt.Sprintf("Heavy func %d", n))
	defer span.End()

	time.Sleep(time.Duration(n) * time.Second)

	// (C) 5秒より処理に時間がかかったらエラーとする
	if n > 5 {
		msg := "timeout!"
		span.RecordError(fmt.Errorf("%s", msg))
		span.SetStatus(codes.Error, msg)
	}
}

func main() {
	ctx := context.Background()

	tp, err := initTracerProvider(ctx)
	if err != nil {
		panic(err)
	}
	defer tp.Shutdown(ctx)
	tracer := tp.Tracer("main")

	// (1) /heavy エンドポイントの定義
	http.Handle("/heavy",
		// (A) スパンでハンドラーをラップ
		otelhttp.NewHandler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// (2) 2秒間スリープ
				time.Sleep(time.Duration(2) * time.Second)

				// (3) 重い処理をする関数を3回呼び出す
				superHeavyFunc(r.Context(), tracer, 8)
				superHeavyFunc(r.Context(), tracer, 3)
				superHeavyFunc(r.Context(), tracer, 5)

				// (4) テキストでレスポンスを返す
				fmt.Fprintln(w, "This is heavy endpoint")
			}),
			"heavy-endpoint",
		),
	)

	fmt.Println("Starting server at :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Server failed:", err)
	}
}
