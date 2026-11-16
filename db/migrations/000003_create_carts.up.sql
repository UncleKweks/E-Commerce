CREATE TABLE carts (
    id         BIGSERIAL   PRIMARY KEY,
    -- UNIQUE: each user has exactly one cart.
    user_id    BIGINT      NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cart_items (
    id         BIGSERIAL   PRIMARY KEY,
    cart_id    BIGINT      NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    product_id BIGINT      NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    quantity   INTEGER     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT cart_items_quantity_positive CHECK (quantity > 0),
    -- Adding the same product twice increases quantity instead of creating a second row.
    CONSTRAINT cart_items_cart_product_key UNIQUE (cart_id, product_id)
);

CREATE INDEX cart_items_product_id_idx ON cart_items (product_id);
