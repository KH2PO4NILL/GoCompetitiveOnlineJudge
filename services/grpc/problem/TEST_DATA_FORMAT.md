# Problem test data package

Problem RPC accepts a ZIP archive whose root contains `manifest.json`.
Every input and output path is relative to the archive root and must name a
regular file inside the same archive.

```json
{
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

Unknown manifest fields, duplicate paths or ordinals, absolute paths,
`..` traversal, symbolic links, missing referenced files and archives beyond
the configured file or uncompressed-size limits are rejected.

After validation, Problem RPC copies the temporary upload to a server-owned
immutable version key before committing the new problem revision. Judge reads
that version object rather than the still-live presigned upload target.
