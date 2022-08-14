# About
Prometheus HTTP SD based on DNS service

# Config example
```
---
provider:
  type: yandex               # only this supported right now
  metadata:
    folderIds:
      - aaaaaaaaaaaaaaaaaaaa # which folders must be listed

rules:
  - path: /node-exporter     # http url path, where targets will be given
    port: 9100               # targets scrape port
    filters:                 # regexps, which will be used to check record names matching
      - one.iv91.ru.
      - '.*.iv91.ru.'
```
