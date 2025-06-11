import 'dotenv/config';
import { app } from './app';

const PORT = process.env.PORT || 3000;
const HOST = process.env.HOST || 'localhost';

app.listen(PORT);

console.log(`
🚀 PayPal Service is running!
📡 Server: http://${HOST}:${PORT}
🔧 Environment: ${process.env.PAYPAL_MODE || 'sandbox'}
📚 API Docs: http://${HOST}:${PORT}/api
🏥 Health Check: http://${HOST}:${PORT}/api/paypal/health

💡 Make sure to configure your PayPal credentials in .env file:
   - PAYPAL_CLIENT_ID
   - PAYPAL_CLIENT_SECRET
   - PAYPAL_MODE (sandbox/production)
`);
