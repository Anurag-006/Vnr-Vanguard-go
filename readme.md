# ⚡ VNR Vanguard

VNR Vanguard is a high-performance, concurrent results engine and analytics dashboard for VNRVJIET students.

Originally written in Python, this system was completely re-architected in Go to solve three major challenges inherent to academic portal scrapers: severe network latency, server memory leaks, and the risk of upstream DDOS during result-day traffic spikes.

---

## 🧠 System Architecture

Vanguard isn't just a scraper; it's a defensive data pipeline. It is engineered to deliver sub-millisecond response times to users while strictly protecting the upstream college servers from being overwhelmed.

### Core Engineering Features

**L1 + L2 Tiered Caching**

- **L1 (RAM):** Hot queries are served directly from local memory in ~0.001 seconds.  
- **L2 (Redis):** If Render spins down the instance, data persists in an Upstash Redis cluster, allowing the app to recover state instantly upon reboot.

**Cache Stampede Protection (singleflight)**

If 100 students request the same batch at the exact same millisecond, Vanguard coalesces them. It fires exactly one network request to the college portal, caches the result, and delivers it to all 100 clients simultaneously.

**Semaphore Concurrency Control**

Worker goroutines are strictly throttled (max 15 concurrent network calls). This ensures rapid data retrieval without triggering IP bans or overloading the college's legacy infrastructure.

**Smart "Withheld" Parsing**

Gracefully handles incomplete or withheld result states without skewing class average SGPAs or failing the scrape.

**Client-Side Compute**

The complex matrix generation for CSV exports is offloaded entirely to the browser using Vanilla JS, keeping the Go backend stateless and lightweight.

---

## ✨ Frontend Features

**Dark Glassmorphism UI**

A sleek, zero-dependency, responsive interface.

**Squad Mode**

Paste a raw WhatsApp chat or comma-separated list of roll numbers to instantly generate a custom mini-leaderboard of your friend group.

**Advanced Analytics**

Calculates valid class averages, pass/fail ratios, and isolates subjects causing the highest backlog rates.

**Full-Matrix CSV Export**

Downloads a complete spreadsheet mapping every student to their exact numeric grade points for every subject.

---

## 🚀 Local Development

To run Vanguard locally, you need **Go 1.22+** installed.

### 1. Clone the repository

```bash
git clone https://github.com/Anurag-006/Vnr-Vanguard-go.git
cd Vnr-Vanguard-go
```

### 2. Setup Environment Variables

Create a `.env` file in the root directory:

```env
# Optional: Speeds up development and saves state across reboots
REDIS_URL=rediss://default:YOUR_PASSWORD@your-endpoint.upstash.io:6379
```

### 3. Install Dependencies and Run

```bash
go mod tidy
go run cmd/vanguard/main.go
```

The application will be live at **http://localhost:8080**.

---

## 🐳 Docker Deployment

This project uses a highly optimized **Multi-Stage Docker build**. It strips debug symbols and omits the Go compiler from the final image, resulting in a tiny, secure runtime container perfectly suited for Render's free tier (512MB RAM).

```bash
docker build -t vanguard-app .
docker run -p 8080:8080 vanguard-app
```

---

## ⚖️ Disclaimer

This is an independent, student-led project. It is **not affiliated with, endorsed by, or connected to VNRVJIET**. The application acts as a read-only data aggregator and implements strict rate-limiting and caching to ensure respectful usage of college network resources.