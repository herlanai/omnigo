# Runtime and native metrics

`NewMonitor` creates an inactive sampler with no native resources. `Run(ctx)` is single-use and owns native initialization/release; cancellation ends sampling. `Snapshot()` returns an independent snapshot safe to read from service goroutines. History is bounded to 120 samples at a 2-second interval.

Go heap, goroutine and GC metrics are available across platforms. macOS additionally samples process CPU/RSS and system CPU/memory through the native adapter. Other platforms report unavailable native metrics explicitly. Studio reads this framework service and provides only its visualization.
