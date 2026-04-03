## Профилирование памяти (pprof)

Сняты профили:

- profiles/base.pprof
- profiles/result.pprof

Проверка:

```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

File: shortener
Build ID: 1bc27970ca32f68cfa19449d9a24ce29f132ad6c
Type: inuse_space
Time: 2026-02-15 17:55:19 MSK
Showing nodes accounting for 14306.44kB, 176.58% of 8101.83kB total
      flat  flat%   sum%        cum   cum%
19334.31kB 238.64% 238.64% 19334.31kB 238.64%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*MemoryRepository).Save
-3328.49kB 41.08% 197.56% -4512.76kB 55.70%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*FileRepository).Save
-1184.27kB 14.62% 182.94% -1184.27kB 14.62%  bytes.growSlice
   -1026kB 12.66% 170.28%    -1026kB 12.66%  bufio.NewWriterSize (inline)
   -1026kB 12.66% 157.61%    -1026kB 12.66%  runtime.allocm
  512.88kB  6.33% 163.94%   512.88kB  6.33%  sync.(*Pool).pinSlow
  512.02kB  6.32% 170.26% 15845.57kB 195.58%  github.com/Dyuzhovsergey/shortener-url/internal/handler.(*HTTPServer).handlePost
  512.01kB  6.32% 176.58%   512.01kB  6.32%  github.com/Dyuzhovsergey/shortener-url/internal/service.(*ShorterService).generateID
         0     0% 176.58% -1184.27kB 14.62%  bytes.(*Buffer).WriteString
         0     0% 176.58% -1184.27kB 14.62%  bytes.(*Buffer).grow
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.(*Encoder).Encode
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.(*encodeState).marshal
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.(*encodeState).reflectValue
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.arrayEncoder.encode
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.sliceEncoder.encode
         0     0% 176.58% -1184.27kB 14.62%  encoding/json.structEncoder.encode
         0     0% 176.58% 15845.57kB 195.58%  github.com/Dyuzhovsergey/shortener-url/internal/handler.(*HTTPServer).Router.ZapLogger.func1.1
         0     0% 176.58% 15845.57kB 195.58%  github.com/Dyuzhovsergey/shortener-url/internal/middleware.AuthMiddleware.func1
         0     0% 176.58% 15845.57kB 195.58%  github.com/Dyuzhovsergey/shortener-url/internal/middleware.GzipMiddleware.func1
         0     0% 176.58% -1184.27kB 14.62%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*FileRepository).rewriteFileLocked
         0     0% 176.58% 15333.56kB 189.26%  github.com/Dyuzhovsergey/shortener-url/internal/service.(*ShorterService).CreateShortURL
         0     0% 176.58% 16358.45kB 201.91%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 176.58% 15845.57kB 195.58%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 176.58%    -1026kB 12.66%  net/http.(*conn).readRequest
         0     0% 176.58% 15332.45kB 189.25%  net/http.(*conn).serve
         0     0% 176.58% 15845.57kB 195.58%  net/http.HandlerFunc.ServeHTTP
         0     0% 176.58%    -1026kB 12.66%  net/http.newBufioWriterSize
         0     0% 176.58% 16358.45kB 201.91%  net/http.serverHandler.ServeHTTP
         0     0% 176.58%     -513kB  6.33%  runtime.mcall
         0     0% 176.58%     -513kB  6.33%  runtime.mstart
         0     0% 176.58%     -513kB  6.33%  runtime.mstart0
         0     0% 176.58%     -513kB  6.33%  runtime.mstart1
         0     0% 176.58%    -1026kB 12.66%  runtime.newm
         0     0% 176.58%     -513kB  6.33%  runtime.park_m
         0     0% 176.58%    -1026kB 12.66%  runtime.resetspinning
         0     0% 176.58%    -1026kB 12.66%  runtime.schedule
         0     0% 176.58%    -1026kB 12.66%  runtime.startm
         0     0% 176.58%    -1026kB 12.66%  runtime.wakep
         0     0% 176.58%   512.88kB  6.33%  sync.(*Pool).Get
         0     0% 176.58%   512.88kB  6.33%  sync.(*Pool).pin
