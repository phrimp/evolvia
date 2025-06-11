import { Money, Item } from '../types/paypal';

/**
 * Format currency amount
 */
export const formatCurrency = (amount: number, currency: string = 'USD'): string => {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: currency
  }).format(amount);
};

/**
 * Validate currency code
 */
export const isValidCurrency = (currency: string): boolean => {
  const validCurrencies = [
    'USD', 'EUR', 'GBP', 'JPY', 'CAD', 'AUD', 'CHF', 'CNY', 'SEK', 'NZD',
    'MXN', 'SGD', 'HKD', 'NOK', 'TRY', 'RUB', 'INR', 'BRL', 'ZAR', 'KRW'
  ];
  return validCurrencies.includes(currency.toUpperCase());
};

/**
 * Calculate total amount from items
 */
export const calculateTotalAmount = (items: Item[]): string => {
  const total = items.reduce((sum, item) => {
    const itemAmount = parseFloat(item.unit_amount.value);
    const quantity = parseInt(item.quantity);
    return sum + (itemAmount * quantity);
  }, 0);
  
  return total.toFixed(2);
};

/**
 * Validate amount format
 */
export const isValidAmount = (amount: string): boolean => {
  const amountRegex = /^\d+(\.\d{1,2})?$/;
  const numAmount = parseFloat(amount);
  return amountRegex.test(amount) && numAmount > 0 && numAmount <= 10000;
};

/**
 * Create Money object
 */
export const createMoney = (value: string, currency: string = 'USD'): Money => {
  return {
    currency_code: currency.toUpperCase(),
    value: parseFloat(value).toFixed(2)
  };
};

/**
 * Validate email format
 */
export const isValidEmail = (email: string): boolean => {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(email);
};

/**
 * Generate order reference ID
 */
export const generateOrderReference = (): string => {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substring(2, 8);
  return `ORDER_${timestamp}_${random}`.toUpperCase();
};

/**
 * Validate PayPal order ID format
 */
export const isValidPayPalOrderId = (orderId: string): boolean => {
  // PayPal order IDs are typically 17 characters long
  const orderIdRegex = /^[A-Z0-9]{17}$/;
  return orderIdRegex.test(orderId);
};

/**
 * Format error message for client
 */
export const formatErrorResponse = (error: any) => {
  return {
    success: false,
    error: error.name || 'PayPal Error',
    message: error.message || 'An error occurred while processing your request',
    details: error.details || null,
    timestamp: new Date().toISOString()
  };
};

/**
 * Log PayPal transaction
 */
export const logTransaction = (type: string, orderId: string, data: any) => {
  console.log(`[PayPal Transaction] ${type}:`, {
    orderId,
    timestamp: new Date().toISOString(),
    data
  });
};

/**
 * Sanitize webhook data for logging
 */
export const sanitizeWebhookData = (data: any) => {
  // Remove sensitive information from webhook data before logging
  const sanitized = { ...data };
  
  // Remove or mask sensitive fields
  if (sanitized.resource?.payer?.email_address) {
    sanitized.resource.payer.email_address = sanitized.resource.payer.email_address.replace(
      /(.{2})(.*)(@.*)/,
      '$1***$3'
    );
  }
  
  return sanitized;
};

/**
 * Create success response format
 */
export const createSuccessResponse = (data: any, message?: string) => {
  return {
    success: true,
    message: message || 'Operation completed successfully',
    data,
    timestamp: new Date().toISOString()
  };
};

/**
 * Check if running in production
 */
export const isProduction = (): boolean => {
  return process.env.NODE_ENV === 'production' || process.env.PAYPAL_MODE === 'production';
};

/**
 * Get base URL for callbacks
 */
export const getBaseUrl = (): string => {
  const host = process.env.HOST || 'http://localhost';
  const port = process.env.PORT || '3000';
  
  if (host.startsWith('http')) {
    return `${host}:${port}`;
  }
  
  return `http://${host}:${port}`;
};
