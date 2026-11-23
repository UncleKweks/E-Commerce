-- Sample catalog for local development. Run with: make seed
-- Safe to run more than once: ON CONFLICT ... DO NOTHING skips rows that exist.
-- This is NOT a migration. Migrations change the structure of the database;
-- seed data is just example content and never goes to production.

INSERT INTO categories (name, description) VALUES
    ('Clothing',    'Shirts, hoodies and more'),
    ('Electronics', 'Gadgets and accessories'),
    ('Books',       'Paperbacks and hardcovers')
ON CONFLICT (lower(name)) WHERE deleted_at IS NULL DO NOTHING;

-- A hidden category, to show that its products don't appear in public lists.
INSERT INTO categories (name, description, is_active) VALUES
    ('Coming Soon', 'Not visible to customers yet', FALSE)
ON CONFLICT (lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO products (category_id, name, description, price_cents, stock, sku, is_active)
SELECT c.id, p.name, p.description, p.price_cents, p.stock, p.sku, p.is_active
FROM (VALUES
    ('clothing',    'Plain T-Shirt',          '100% cotton, unisex',          1999,  50, 'TSHIRT-PLAIN',   TRUE),
    ('clothing',    'Graphic T-Shirt',        'Printed front, 100% cotton',   2499,  20, 'TSHIRT-GRAPHIC', TRUE),
    ('clothing',    'Hoodie',                 'Fleece-lined, with pocket',    4999,  10, 'HOODIE-BASIC',   TRUE),
    ('clothing',    'Wool Socks',             'Pack of 3',                     999,   0, 'SOCKS-WOOL-3',   TRUE),
    ('electronics', 'Wireless Mouse',         '2.4 GHz, USB receiver',        2999,  35, 'MOUSE-WL',       TRUE),
    ('electronics', 'Mechanical Keyboard',    'Brown switches',               8999,   5, 'KEYB-MECH',      TRUE),
    ('electronics', 'USB-C Cable',            '1 metre, braided',              899, 200, 'CABLE-USBC-1M',  TRUE),
    ('electronics', 'Noise-Cancelling Headphones', 'Over-ear, 30h battery',  15999,   3, 'HEADPHONES-NC',  TRUE),
    ('electronics', 'Old Phone Charger',      'Discontinued',                  499,  12, 'CHARGER-OLD',    FALSE),
    ('books',       'Learning Go',            'An idiomatic approach',        3999,  15, 'BOOK-LEARN-GO',  TRUE),
    ('books',       'Designing Data-Intensive Applications', 'Databases, explained', 4599, 8, 'BOOK-DDIA', TRUE),
    ('books',       'The Pragmatic Programmer', '20th anniversary edition',   4299,   0, 'BOOK-PRAGPROG',  TRUE),
    ('coming soon', 'Smart Watch',            'Launching next month',        19999, 100, 'WATCH-SMART',    TRUE)
) AS p (category, name, description, price_cents, stock, sku, is_active)
JOIN categories c ON lower(c.name) = p.category AND c.deleted_at IS NULL
ON CONFLICT (sku) WHERE deleted_at IS NULL DO NOTHING;
