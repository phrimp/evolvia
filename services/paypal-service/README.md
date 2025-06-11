# PayPal Service

Dịch vụ PayPal integration sử dụng Elysia framework và PayPal Checkout Server SDK.

## 🚀 Quick Start

### 1. Cài đặt dependencies

```bash
bun install
```

### 2. Cấu hình môi trường

Sao chép file `.env` và cập nhật thông tin PayPal của bạn:

```env
# PayPal Configuration
PAYPAL_CLIENT_ID=your_paypal_client_id_here
PAYPAL_CLIENT_SECRET=your_paypal_client_secret_here
PAYPAL_MODE=sandbox  # hoặc 'production' cho live

# Server Configuration
PORT=3000
HOST=localhost
```

### 3. Lấy PayPal credentials

1. Truy cập [PayPal Developer](https://developer.paypal.com/)
2. Tạo application mới
3. Copy Client ID và Client Secret
4. Paste vào file `.env`

### 4. Chạy service

```bash
# Development mode
bun run dev

# Production mode
bun start
```

## 📚 API Documentation

### Base URL

- Development: `http://localhost:3000`
- Health Check: `GET /api/paypal/health`

### Endpoints

#### 1. Tạo PayPal Order

```http
POST /api/paypal/create-order
Content-Type: application/json

{
  "amount": "100.00",
  "currency": "USD",
  "description": "Test purchase",
  "items": [
    {
      "name": "Product Name",
      "unit_amount": {
        "currency_code": "USD",
        "value": "100.00"
      },
      "quantity": "1",
      "description": "Product description"
    }
  ]
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "orderId": "7XX123456789012345",
    "status": "CREATED",
    "approvalUrl": "https://www.sandbox.paypal.com/checkoutnow?token=7XX123456789012345"
  }
}
```

#### 2. Capture PayPal Order

```http
POST /api/paypal/capture-order/{orderId}
```

#### 3. Get Order Details

```http
GET /api/paypal/order/{orderId}
```

#### 4. Callback URLs

- Success: `GET /api/paypal/success?token={orderId}&PayerID={payerId}`
- Cancel: `GET /api/paypal/cancel?token={orderId}`

#### 5. Webhook Handler

```http
POST /api/paypal/webhook
```

## 🔄 Payment Flow

1. **Tạo Order**: Client gọi `POST /api/paypal/create-order`
2. **Redirect**: Client redirect user đến `approvalUrl`
3. **User Payment**: User thực hiện thanh toán trên PayPal
4. **Callback**: PayPal redirect về `/api/paypal/success` hoặc `/api/paypal/cancel`
5. **Capture**: Server tự động capture payment khi success
6. **Webhook**: PayPal gửi webhook events về `/api/paypal/webhook`

## 🛠️ Usage Examples

### JavaScript/TypeScript Client

```javascript
// Tạo PayPal order
const createOrder = async () => {
  const response = await fetch(
    "http://localhost:3000/api/paypal/create-order",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        amount: "50.00",
        currency: "USD",
        description: "Test purchase",
      }),
    }
  );

  const data = await response.json();

  if (data.success) {
    // Redirect user to PayPal
    window.location.href = data.data.approvalUrl;
  }
};
```

## 🔒 Security & Production

- ✅ CORS configured
- ✅ Environment variables for credentials
- ✅ Error handling and validation
- ✅ Webhook signature validation
- ✅ Request logging

## 📞 Support

- PayPal Developer Documentation: https://developer.paypal.com/docs/
- PayPal Sandbox: https://developer.paypal.com/developer/accounts/

Open http://localhost:3000/ with your browser to see the result.
