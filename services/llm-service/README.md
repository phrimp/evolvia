# LLM Service - Evolvia Platform

Dịch vụ LLM tích hợp RAG (Retrieval-Augmented Generation) cho nền tảng Evolvia, hỗ trợ chat thông minh với khả năng truy xuất thông tin từ database.

## ✨ Tính năng chính

- **Chat Session Management**: Tạo và quản lý phiên chat
- **RAG Integration**: Truy xuất thông tin người dùng và đơn hàng từ MongoDB
- **Security Guard**: Bảo vệ khỏi các truy vấn không phù hợp
- **JWT Authentication**: Hỗ trợ xác thực người dùng
- **RabbitMQ Integration**: Giao tiếp với các service khác
- **Multiple LLM Providers**: Hỗ trợ Ollama, OpenAI, và các provider khác

## 🚀 Quick Start

### 1. Cài đặt dependencies

```bash
go mod tidy
go mod download
```

### 2. Cấu hình môi trường

Cập nhật file `.env`:

```env
# Server Configuration
PORT=8080
GIN_MODE=debug

# MongoDB Configuration
MONGO_URI=mongodb://root:example@ssh.phrimp.io.vn:27017
MONGO_DATABASE=evolvia

# RabbitMQ Configuration
RABBITMQ_URI=amqp://guest:guest@localhost:5672

# LLM Configuration
API_KEY=none
BASE_URL=http://localhost:11434/v1
MODEL=qwen3:1.7b
PROVIDER=ollama

# JWT Configuration
JWT_SECRET=your-jwt-secret-key
```

### 3. Chạy service

```bash
go run main.go
```

Service sẽ chạy tại `http://localhost:8080`

## 📡 API Endpoints

### Public Endpoints

#### Health Check

```
GET /health
```

#### Service Status

```
GET /public/llm/ping
GET /public/llm/model
```

#### Chat Session Management

```
POST /public/llm/model/session
```

Tạo phiên chat mới. Trả về `sessionId`.

#### Chat với AI

```
POST /public/llm/model/:sessionId/chat
```

**Request Body:**

```json
{
  "message": "Tên tôi là gì?",
  "context": {
    "additional": "data"
  }
}
```

**Response:**

```json
{
  "success": true,
  "message": "Chat message processed",
  "data": {
    "message": "Tên của bạn là Thịnh",
    "sessionId": "session_id",
    "timestamp": "2025-01-01T00:00:00Z",
    "sources": []
  }
}
```

#### Lịch sử chat

```
GET /public/llm/model/history/:sessionId
```

### Protected Endpoints

Các endpoint yêu cầu JWT token trong header:

```
Authorization: Bearer <jwt_token>
```

#### Get User Sessions

```
GET /protected/llm/user/sessions?limit=20
```

Lấy danh sách tất cả session chat của user hiện tại.

**Query Parameters:**

- `limit` (optional): Số lượng session tối đa trả về (mặc định: 20, tối đa: 100)

**Response:**

```json
{
  "success": true,
  "message": "User sessions retrieved successfully",
  "data": {
    "userId": "user123",
    "sessions": [
      {
        "id": "ObjectId",
        "sessionId": "session_uuid",
        "userId": "user123",
        "title": "Chat về sản phẩm",
        "createdAt": "2025-01-01T00:00:00Z",
        "updatedAt": "2025-01-01T00:30:00Z",
        "isActive": true
      }
    ],
    "count": 5,
    "limit": 20
  }
}
```

## 🔐 Authentication

### Sử dụng JWT Token

Để truy cập thông tin cá nhân, truyền JWT token:

```bash
curl -X POST http://localhost:8080/public/llm/model/session_123/chat \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "Tôi có bao nhiêu đơn hàng?"}'
```

### Anonymous Chat

Không cần token để chat cơ bản:

```bash
curl -X POST http://localhost:8080/public/llm/model/session_123/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Xin chào!"}'
```

## 🧠 RAG System

### Supported Queries

Khi có JWT token, hệ thống có thể trả lời:

✅ **Thông tin cá nhân:**

- "Tên tôi là gì?"
- "Email của tôi là gì?"
- "Thông tin cá nhân của tôi"

✅ **Đơn hàng:**

- "Tôi có bao nhiêu đơn hàng?"
- "Đơn hàng gần nhất của tôi"
- "Tôi đã mua gì?"

✅ **Hỗ trợ chung:**

- Câu hỏi về dịch vụ Evolvia
- Hướng dẫn sử dụng

### Security Guard

❌ **Từ chối trả lời:**

- Làm bài tập, homework
- Viết code
- Tư vấn đầu tư, chứng khoán
- Vấn đề y tế, pháp lý
- Tin tức, chính trị

## 🗄️ Database Schema

### Collections

#### chat_sessions

```json
{
  "_id": "ObjectId",
  "userId": "string",
  "createdAt": "Date",
  "updatedAt": "Date",
  "title": "string",
  "active": "boolean"
}
```

#### chat_messages

```json
{
  "_id": "ObjectId",
  "sessionId": "string",
  "userId": "string",
  "content": "string",
  "role": "user|assistant",
  "timestamp": "Date",
  "context": "object"
}
```

#### users (for RAG)

```json
{
  "_id": "ObjectId",
  "userId": "string",
  "name": "string",
  "email": "string",
  "profile": {
    "firstName": "string",
    "lastName": "string",
    "avatar": "string",
    "phone": "string"
  }
}
```

#### orders (for RAG)

```json
{
  "_id": "ObjectId",
  "userId": "string",
  "orderId": "string",
  "products": [
    {
      "productId": "string",
      "name": "string",
      "quantity": "number",
      "price": "number"
    }
  ],
  "total": "number",
  "status": "string",
  "createdAt": "Date"
}
```

## 🔧 Development

### Project Structure

```
llm-service/
├── main.go              # Entry point
├── configs/
│   └── config.go       # Configuration management
├── controllers/
│   └── llm_controller.go # HTTP handlers
├── services/
│   ├── database.go     # MongoDB operations
│   ├── rabbitmq.go     # RabbitMQ messaging
│   └── llm.go          # LLM integration
├── models/
│   └── models.go       # Data structures
├── utils/
│   ├── jwt.go          # JWT utilities
│   └── response.go     # HTTP response helpers
├── docs/
│   └── src.txt         # API documentation
├── rag/
│   ├── database.md     # RAG database instructions
│   ├── guard.md        # Security guard prompts
│   └── prompt.md       # System prompts
└── README.md          # This file
```

### Running Tests

```bash
go test ./...
```

### Building for Production

```bash
go build -o llm-service main.go
```

## 🐳 Docker

```bash
# Build image
docker build -t llm-service .

# Run container
docker run -p 8080:8080 --env-file .env llm-service
```

## 📊 Monitoring

### Health Checks

```bash
# Service health
curl http://localhost:8080/health

# Service status with connection info
curl http://localhost:8080/public/llm/model
```

### RabbitMQ Events

Service publishes events:

- `session_created`: Khi tạo phiên chat mới
- `chat_message`: Khi xử lý tin nhắn chat

## 🚨 Troubleshooting

### Common Issues

1. **MongoDB connection failed**

   - Kiểm tra `MONGO_URI` trong `.env`
   - Đảm bảo MongoDB đang chạy

2. **RabbitMQ connection failed**

   - Kiểm tra `RABBITMQ_URI` trong `.env`
   - Đảm bảo RabbitMQ đang chạy

3. **LLM model not responding**

   - Kiểm tra `BASE_URL` và `MODEL` trong `.env`
   - Đảm bảo Ollama/OpenAI đang hoạt động

4. **JWT token invalid**
   - Kiểm tra `JWT_SECRET` trong `.env`
   - Đảm bảo token được tạo đúng cách

## 📚 Additional Resources

- [Gin Documentation](https://gin-gonic.com/)
- [MongoDB Go Driver](https://docs.mongodb.com/drivers/go/)
- [RabbitMQ Go Client](https://github.com/streadway/amqp)
- [Ollama API](https://github.com/ollama/ollama/blob/main/docs/api.md)

## 📝 License

This project is part of the Evolvia platform.
