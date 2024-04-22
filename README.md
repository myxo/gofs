Features:
- performance in non mock path (no interfaces)
- profound testing (we use property based tsting to ensure simularity with real implementation

Non features:
- make a varaity of different backends (like NetFS, google cloud, s3, etc.). I try to keep package as clean from dependencies as possible
- simulating of concurrent effect of filesystem (e.g. concurrent ReadDir with file removing)

TODO:
- [ ] Make count in test to see how much function envocation we have
- [ ] Document what fileMode are supported
- [ ] O_APPEND
- [ ] Fallocate?
- [ ] copy paste docs from orig functions
- [ ] subdirs in tests
- [ ] CI with test and fmtcheck
- [ ] Use more stdlib errors (how to test this?)
- [ ] Test relative paths
