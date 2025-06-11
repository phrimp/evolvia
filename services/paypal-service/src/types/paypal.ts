export interface PayPalOrder {
  id: string;
  status: string;
  links: PayPalLink[];
  purchase_units: PurchaseUnit[];
  payer?: Payer;
  create_time?: string;
  update_time?: string;
}

export interface PayPalLink {
  href: string;
  rel: string;
  method: string;
}

export interface PurchaseUnit {
  reference_id?: string;
  amount: Amount;
  description?: string;
  items?: Item[];
  shipping?: Shipping;
}

export interface Amount {
  currency_code: string;
  value: string;
  breakdown?: AmountBreakdown;
}

export interface AmountBreakdown {
  item_total?: Money;
  shipping?: Money;
  handling?: Money;
  tax_total?: Money;
  insurance?: Money;
  shipping_discount?: Money;
  discount?: Money;
}

export interface Money {
  currency_code: string;
  value: string;
}

export interface Item {
  name: string;
  unit_amount: Money;
  tax?: Money;
  quantity: string;
  description?: string;
  sku?: string;
  category?: 'DIGITAL_GOODS' | 'PHYSICAL_GOODS';
}

export interface Shipping {
  name?: Name;
  address?: Address;
}

export interface Name {
  full_name?: string;
  given_name?: string;
  surname?: string;
}

export interface Address {
  address_line_1?: string;
  address_line_2?: string;
  admin_area_2?: string; // City
  admin_area_1?: string; // State
  postal_code?: string;
  country_code: string;
}

export interface Payer {
  name?: Name;
  email_address?: string;
  payer_id?: string;
  address?: Address;
}

export interface CreateOrderRequest {
  amount: string;
  currency?: string;
  description?: string;
  items?: Item[];
  returnUrl?: string;
  cancelUrl?: string;
}

export interface CaptureOrderResponse {
  id: string;
  status: string;
  payment_source?: any;
  purchase_units: PurchaseUnit[];
  payer: Payer;
  links: PayPalLink[];
}

export interface PayPalError {
  error: string;
  error_description?: string;
  details?: Array<{
    field?: string;
    value?: string;
    location?: string;
    issue: string;
    description?: string;
  }>;
}
