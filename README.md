# 🌀 **Breeze** — High-Performance Golang Web Framework

[![Go Reference](https://pkg.go.dev/badge/github.com/nelthaarion/breeze.svg)](https://pkg.go.dev/github.com/nelthaarion/breeze)
[![Go Report Card](https://goreportcard.com/badge/github.com/nelthaarion/breeze)](https://goreportcard.com/report/github.com/nelthaarion/breeze)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Breeze** is a **fast, lightweight, and modern Go web framework** designed for simplicity, flexibility, and high performance. Build secure, maintainable web applications with ease. 🌟

---

## ✨ Features

* 🏎️ **High-performance routing**
* 🛡️ **Middleware support** for auth, security, compression, caching, CORS, rate limiting, etc.
* 📦 Helpers for **JSON, HTML, string responses**
* 🔐 **Security headers middleware** (CSP, HSTS, XSS protection)
* 🔑 **JWT authentication & refresh token support**
* ⏱️ **Rate limiting** & **throttling**
* ♻️ **Panic recovery** to keep your server alive
* 📈 Optional **ETag / in-memory caching**

---

## 🚀 Installation

```bash
go get github.com/nelthaarion/breeze
```

---

## 🏁 Quick Start

```go
package main

import (
	"github.com/nelthaarion/breeze"
	"github.com/nelthaarion/breeze/middleware"
)

func main() {
	router := breeze.NewRouter()

	// 🌐 Global security middleware
	router.Use(middleware.DefaultSecurityMiddleware())

	// 🔐 Route-specific middleware: JWT authentication
	router.Handle("GET", "/profile", profileHandler, middleware.JWTAuthMiddleware(middleware.JWTOptions{
		AccessSecret:       "access_secret",
		RefreshSecret:      "refresh_secret",
		EnableRefreshToken: true,
		RequiredRoles:      []string{"user", "admin"},
	}))

	// 🚀 Start server
	router.Listen(":8080")
}

func profileHandler(ctx *breeze.Context) {
	user := ctx.GetParam("user") // claims set by JWT middleware
	ctx.JSON(map[string]string{
		"message": "Welcome " + user + " 🌟",
	})
}
```

---

## 🧩 Middleware

Middlewares in Breeze are **just `HandlerFunc`s**. They can be applied:

* **Globally** with `router.Use()`
* **Per-route** with `router.Handle()`

### 🔹 Built-in Middlewares

| Middleware         | Description                                                             |
| ------------------ | ----------------------------------------------------------------------- |
| 🛡️ Security       | Adds headers like CSP, HSTS, X-Frame-Options, XSS-Protection            |
| 🔑 JWT Auth        | Validates access & refresh tokens, supports roles and claims validation |
| ♻️ Recovery        | Catches panics in handlers or middleware                                |
| ⏱️ Rate Limiting   | Limits requests per client IP                                           |
| 🌍 CORS            | Handles cross-origin requests                                           |
| 🗜️ Compression    | Supports gzip, deflate, brotli                                          |
| 🏷️ ETag / Caching | Adds ETag, conditional GET, optional in-memory caching                  |

---

## 👨‍💻 User Use Case: Middleware Chaining

Imagine a **profile endpoint** that requires:

1. Authenticated user (JWT)
2. Safe security headers
3. Panic-safe execution

```go
router.Handle("GET", "/profile", profileHandler,
	middleware.RecoveryMiddleware(),
	middleware.DefaultSecurityMiddleware(),
	middleware.JWTAuthMiddleware(middleware.JWTOptions{
		AccessSecret:  "access_secret",
		RefreshSecret: "refresh_secret",
		RequiredRoles: []string{"user", "admin"},
	}),
)
```

**Flow**:

1. ♻️ `RecoveryMiddleware()` – prevents crashes
2. 🛡️ `DefaultSecurityMiddleware()` – adds headers
3. 🔑 `JWTAuthMiddleware()` – validates tokens, sets claims in `ctx.params`

Inside `profileHandler`:

```go
func profileHandler(ctx *breeze.Context) {
	user := ctx.GetParam("user") // safely access JWT claims
	ctx.JSON(map[string]string{
		"message": "Welcome " + user + " 🌟",
	})
}
```

---

## 💡 Tips & Tricks

* Use **`ctx.SetParam`** & **`ctx.GetParam`** to safely store/retrieve per-request data
* Chain multiple middlewares for **fine-grained control**
* Combine **global and per-route middleware** for flexible security policies
* Use **refresh tokens** to automatically renew JWT access tokens

---

## 🎨 Summary

Breeze makes Go web development:

* ✅ Fast
* ✅ Secure
* ✅ Extensible
* ✅ Fun

Build your next Go API, microservice, or web app **the Breeze way**! 🌬️


## 🧑‍💻 Contributing

Contributions are welcome!
Please open issues and pull requests for new features, bug fixes, and optimizations.

---

## 📄 License

MIT License © 2025 Farhsad Khazaei Fard(https://github.com/nelthaarion)
