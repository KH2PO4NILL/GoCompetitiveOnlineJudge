# Problem test data package

Problem RPC accepts a ZIP archive whose root contains `manifest.json`.
Both Problem RPC and Judge Worker use the versioned schema implemented by
`pkg/testdatamanifest`. Every payload file must be declared exactly once; every
input and output path is relative to the archive root and must name a declared
regular file.

```json
{
  "version": 1,
  "files": [
    {
      "path": "cases/1.in",
      "sha256": "a1f2...64 hexadecimal characters...9c",
      "size": 4
    },
    {
      "path": "cases/1.out",
      "sha256": "7b4e...64 hexadecimal characters...10",
      "size": 2
    }
  ],
  "test_cases": [
    {
      "name": "case-1",
      "input_file": "cases/1.in",
      "output_file": "cases/1.out",
      "score": 100,
      "ordinal": 1,
      "enabled": true,
      "group_name": "main"
    }
  ]
}
```

`version` is currently `1`. SHA-256 values are hexadecimal digests of the
uncompressed file contents and `size` is the exact uncompressed byte count.
The manifest does not list itself.

Unknown fields, unsupported versions, duplicate paths or ordinals, undeclared
payload files, absolute paths, non-canonical paths, `..` traversal, links,
special files, missing referenced files, false sizes/checksums, excessive file
counts or sizes, and excessive compression ratios are rejected.

The manifest intentionally does not contain the ZIP package checksum: an archive
cannot reliably contain its own digest. For each judge request, Worker requires
the returned `JudgeProblemSpec.revision` to equal the requested revision, streams
the downloaded ZIP through SHA-256 and compares it with
`JudgeProblemSpec.test_data_checksum`, then requires the manifest test-case
metadata to match the same spec by ordinal.

After validation, Problem RPC copies the temporary upload to a server-owned
immutable version key before committing the new problem revision. Judge Worker
uses a separate read-only object-storage identity to GET that immutable object,
safely extracts it, verifies every extracted file again, and atomically publishes
the checksum-keyed local cache entry.
