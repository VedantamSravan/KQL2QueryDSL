# KQL2QueryDSL
KQL2QueryDSL is a Go-based converter that translates Kibana Query Language (KQL) into equivalent Elasticsearch Query DSL format.


# KQL2QueryDSL

> Convert Kibana Query Language (KQL) to Elasticsearch Query DSL using Go.

KQL2QueryDSL is a lightweight and extensible Go library designed to parse and convert Kibana's human-friendly query syntax into the JSON-based Elasticsearch Query DSL. Its ideal for developers working with Elasticsearch APIs or building custom dashboards that need dynamic query translation.

---

## 🚀 Features

- ✅ Convert basic and nested KQL expressions to Query DSL
- ✅ Supports `AND`, `OR`, `NOT`, and grouped conditions
- ✅ Handles match, wildcard, range, and field-based expressions
- ✅ Avoids infinite recursion with safety guards
- ✅ Simple integration into Go applications or web services

---

## 📦 Installation

```bash
go get github.com/VedantamSravan/KQL2QueryDSL
