# SMSpit

**The Mailpit of SMS Testing** - A modern, self-hosted SMS testing server for development.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Why SMSpit?

Email has [Mailpit](https://github.com/axllent/mailpit). SMS had... nothing. Until now.

SMSpit captures SMS messages sent by your application during development, letting you:
- **View messages** in a beautiful web UI
- **Search and filter** by phone number, content, or time
- **Integrate with any SMS-sending code** via simple HTTP webhook
- **Test Kratos/Ory flows** out of the box
- **Persist messages** across restarts (SQLite)
- **Get real-time updates** via WebSocket

## Quick Start

### Docker (Recommended)

```bash
docker run -d -p 8080:8080 -p 9080:9080 ghcr.io/substrate-app/smspit:latest
```

- Web UI: http://localhost:8080
- Webhook endpoint: http://localhost:9080/send

### Docker Compose

```yaml
services:
  smspit:
    image: ghcr.io/substrate-app/smspit:latest
    ports:
      - "8080:8080"   # Web UI
      - "9080:9080"   # Webhook API
    volumes:
      - smspit-data:/data
    environment:
      SMSPIT_DB_PATH: /data/smspit.db

volumes:
  smspit-data:
```

### Kubernetes

```bash
kubectl apply -f https://raw.githubusercontent.com/substrate-app/smspit/main/deploy/kubernetes.yaml
```

## Integrating with Your App

### Simple HTTP Webhook

Send a POST request to capture an SMS:

```bash
curl -X POST http://localhost:9080/send \
  -H "Content-Type: application/json" \
  -d '{"to": "+15551234567", "body": "Your verification code is 123456"}'
```

### Kratos / Ory Integration

Configure Kratos to use SMSpit for SMS:

```yaml
courier:
  sms:
    enabled: true
    request_config:
      url: http://smspit:9080/send
      method: POST
      body: base64://eyJ0byI6Int7IC50byB9fSIsImJvZHkiOiJ7eyAuYm9keSB9fSJ9
      # Decoded: {"to":"{{ .to }}","body":"{{ .body }}"}
```

### Twilio-Compatible Mode

SMSpit can emulate Twilio's API for drop-in replacement:

```bash
# Start with Twilio compatibility
docker run -d -p 8080:8080 -p 9080:9080 \
  -e SMSPIT_TWILIO_COMPAT=true \
  ghcr.io/substrate-app/smspit:latest
```

Then use your existing Twilio code:

```javascript
// Your existing code works unchanged!
const client = require('twilio')('test', 'test');
client.messages.create({
  body: 'Hello!',
  to: '+15551234567',
  from: '+15550001111'
});
// Just point TWILIO_API_URL to http://localhost:9080
```

## Web UI Features

- 📱 **Message List** - All captured SMS with sender, recipient, timestamp
- 🔍 **Search** - Full-text search across all messages
- 🏷️ **Filtering** - Filter by phone number, date range, or tags
- 📊 **Stats** - Message count, recent activity graphs
- 🌙 **Dark Mode** - Easy on the eyes
- 📲 **Mobile Responsive** - Works on any device
- ⚡ **Real-time** - New messages appear instantly (WebSocket)

## API Reference

### Send SMS (Capture)

```http
POST /send
Content-Type: application/json

{
  "to": "+15551234567",
  "from": "+15550009999",  // optional
  "body": "Your code is 123456",
  "tags": ["verification", "kratos"]  // optional
}
```

Response:
```json
{
  "id": "msg_abc123",
  "status": "captured",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### List Messages

```http
GET /api/v1/messages?limit=50&offset=0
```

### Search Messages

```http
GET /api/v1/messages/search?q=verification&to=+1555
```

### Get Single Message

```http
GET /api/v1/messages/{id}
```

### Delete Messages

```http
DELETE /api/v1/messages        # Delete all
DELETE /api/v1/messages/{id}   # Delete one
```

### WebSocket (Real-time)

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('New SMS:', message);
};
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SMSPIT_DB_PATH` | `./smspit.db` | SQLite database path |
| `SMSPIT_WEB_PORT` | `8080` | Web UI port |
| `SMSPIT_API_PORT` | `9080` | Webhook API port |
| `SMSPIT_MAX_MESSAGES` | `10000` | Max messages to retain |
| `SMSPIT_TWILIO_COMPAT` | `false` | Enable Twilio API compatibility |
| `SMSPIT_AUTH_TOKEN` | `` | Optional API authentication |
| `SMSPIT_CORS_ORIGINS` | `*` | Allowed CORS origins |

## Comparison with Alternatives

| Feature | SMSpit | Mock Server | Twilio Test | Mailosaur |
|---------|--------|-------------|-------------|-----------|
| Self-hosted | ✅ | ✅ | ❌ | ❌ |
| Free | ✅ | ✅ | ✅ | ❌ |
| Web UI | ✅ | ❌ | ❌ | ✅ |
| Persistent | ✅ | ❌ | N/A | ✅ |
| Real-time | ✅ | ❌ | ❌ | ✅ |
| Search | ✅ | ❌ | ❌ | ✅ |
| Twilio compat | ✅ | ❌ | ✅ | ❌ |
| Open source | ✅ | ✅ | ❌ | ❌ |

## Use Cases

### 1. Local Development
Test SMS verification flows without sending real messages or paying for SMS.

### 2. CI/CD Testing
Run SMS integration tests in your pipeline with a lightweight container.

### 3. Demo Environments
Show stakeholders how SMS features work without real phone numbers.

### 4. Security Testing
Capture and inspect SMS content for security audits.

## Built With

- **Go** - Fast, single binary, minimal dependencies
- **SQLite** - Embedded database, zero configuration
- **WebSocket** - Real-time updates
- **Tailwind CSS** - Beautiful, responsive UI

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - See [LICENSE](LICENSE) for details.

---

**Made with ❤️ by the [Substrate](https://substrate-platform.com) team**

*SMSpit is part of the Substrate ecosystem but works standalone with any application.*

