-- Add play_product_id to products table to support Google Play Billing for individual products
ALTER TABLE products ADD COLUMN play_product_id TEXT;
