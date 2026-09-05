# Judge Engine

`Engine.Judge(ctx, JudgeInput)` accepts a fixed Problem spec, a cache-pinned verified
test-data package, source files and a Registry Profile ID. It has no MQ/lease dependency.
The caller retains and releases the cache package; Engine owns and cleans the Attempt.

Compilation runs once. Every enabled case runs in its own container, in ascending
ordinal / test-case-ID order. User failures do not short-circuit later cases. Non-CE
outcomes include every enabled case, including SE placeholders on cancellation or
infrastructure errors; a non-nil error is retryable by the orchestration layer.

Standard modes: `exact` compares bytes; `trailing_whitespace` ignores horizontal
whitespace at line ends, CRLF differences and trailing blank lines (not internal or
leading whitespace); `tokens` compares nonempty ASCII-whitespace-delimited byte tokens.
Readers use bounded 32 KiB buffers and a 512 MiB input ceiling, including long tokens.
The default is `tokens`; SPJ and interactive specs are rejected.

Verdict precedence matches Submit RPC: SE > RE > MLE > TLE > OLE > WA > AC.
Only AC cases receive their declared score. CPU milliseconds and memory KiB round up.
Compile diagnostics are UTF-8-safe and limited to 1 MiB. Progress reports compile/run/done
with completed/total counts. The policy digest binds the Problem ID/revision and checker
mode as well as the immutable language policy and effective limits.
