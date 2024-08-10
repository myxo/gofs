Yet another file system mock library in go

Features:
- performance in non mock path (no interfaces)
- profound testing (we use property based tsting to ensure simularity with real implementation)

Non features:
- make a variety of different backends (NetFS, google cloud, s3, etc.). I try to keep this package as clean from dependencies as possible
- simulating of concurrent effect of filesystem (e.g. concurrent ReadDir with file removing in different goroutine)

TODO:
- [ ] Document what fileMode are supported
- [ ] O_APPEND
- [ ] Fallocate?
- [ ] copy paste docs from orig functions
- [ ] add mode there instead on std error we get stacktrace inside error?
- [ ] add thread safe fs?
- [ ] add benchmark to track allocation
- [ ] make good readme file
- [ ] try to add fuzzing (and try to introduce errors)
- [ ] add example of extension (compressed reader?)
