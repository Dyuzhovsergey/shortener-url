## Профилирование памяти (pprof)

Сняты профили:

- profiles/base.pprof
- profiles/result.pprof

Проверка:

```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

File: shortener
Build ID: e3100cafab2fc34a0af313208f7a1350d212bfd9
Type: inuse_space
Time: 2026-02-15 17:55:19 MSK
Showing nodes accounting for -1934.04kB, 23.87% of 8101.83kB total
      flat  flat%   sum%        cum   cum%
 4116.78kB 50.81% 50.81%  4116.78kB 50.81%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*FileRepository).applyRecordLocked
-3328.49kB 41.08%  9.73% -4512.76kB 55.70%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*FileRepository).Save
-1184.27kB 14.62%  4.89% -1184.27kB 14.62%  bytes.growSlice
   -1026kB 12.66% 17.55%    -1026kB 12.66%  bufio.NewWriterSize (inline)
 -512.05kB  6.32% 23.87%  -512.05kB  6.32%  time.NewTicker
 -512.02kB  6.32% 30.19%  -512.02kB  6.32%  encoding/hex.EncodeToString (inline)
  512.02kB  6.32% 23.87%   512.02kB  6.32%  encoding/json.(*decodeState).literalStore
         0     0% 23.87% -1184.27kB 14.62%  bytes.(*Buffer).WriteString
         0     0% 23.87% -1184.27kB 14.62%  bytes.(*Buffer).grow
         0     0% 23.87%   512.02kB  6.32%  encoding/json.(*Decoder).Decode
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.(*Encoder).Encode
         0     0% 23.87%   512.02kB  6.32%  encoding/json.(*decodeState).array
         0     0% 23.87%   512.02kB  6.32%  encoding/json.(*decodeState).object
         0     0% 23.87%   512.02kB  6.32%  encoding/json.(*decodeState).unmarshal
         0     0% 23.87%   512.02kB  6.32%  encoding/json.(*decodeState).value
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.(*encodeState).marshal
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.(*encodeState).reflectValue
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.arrayEncoder.encode
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.sliceEncoder.encode
         0     0% 23.87% -1184.27kB 14.62%  encoding/json.structEncoder.encode
         0     0% 23.87% -5024.77kB 62.02%  github.com/Dyuzhovsergey/shortener-url/internal/handler.(*HTTPServer).Router.ZapLogger.func1.1
         0     0% 23.87% -4512.76kB 55.70%  github.com/Dyuzhovsergey/shortener-url/internal/handler.(*HTTPServer).handlePost
         0     0% 23.87% -5024.77kB 62.02%  github.com/Dyuzhovsergey/shortener-url/internal/middleware.AuthMiddleware.func1
         0     0% 23.87% -5024.77kB 62.02%  github.com/Dyuzhovsergey/shortener-url/internal/middleware.GzipMiddleware.func1
         0     0% 23.87%  -512.02kB  6.32%  github.com/Dyuzhovsergey/shortener-url/internal/middleware.generateUserID
         0     0% 23.87% -1184.27kB 14.62%  github.com/Dyuzhovsergey/shortener-url/internal/repository.(*FileRepository).rewriteFileLocked
         0     0% 23.87%  4628.79kB 57.13%  github.com/Dyuzhovsergey/shortener-url/internal/repository.NewFileRepository
         0     0% 23.87% -4512.76kB 55.70%  github.com/Dyuzhovsergey/shortener-url/internal/service.(*ShorterService).CreateShortURL
         0     0% 23.87%  -512.05kB  6.32%  github.com/Dyuzhovsergey/shortener-url/internal/service.(*ShorterService).deleteWorker
         0     0% 23.87% -5024.77kB 62.02%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 23.87% -4512.76kB 55.70%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 23.87%  4628.79kB 57.13%  main.main
         0     0% 23.87%    -1026kB 12.66%  net/http.(*conn).readRequest
         0     0% 23.87% -6050.78kB 74.68%  net/http.(*conn).serve
         0     0% 23.87% -5024.77kB 62.02%  net/http.HandlerFunc.ServeHTTP
         0     0% 23.87%    -1026kB 12.66%  net/http.newBufioWriterSize
         0     0% 23.87% -5024.77kB 62.02%  net/http.serverHandler.ServeHTTP
         0     0% 23.87%  4628.79kB 57.13%  runtime.main
         0     0% 23.87%     -513kB  6.33%  runtime.mcall
         0     0% 23.87%      513kB  6.33%  runtime.mstart
         0     0% 23.87%      513kB  6.33%  runtime.mstart0
         0     0% 23.87%      513kB  6.33%  runtime.mstart1
         0     0% 23.87%     -513kB  6.33%  runtime.park_m
