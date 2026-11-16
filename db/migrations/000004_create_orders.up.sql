-- Orders are financial records: they are never deleted, soft or otherwise.
CREATE TABLE orders (
    id          BIGSERIAL   PRIMARY KEY,
    -- RESTRICT: a user with orders can't be hard-deleted, which protects the order history.
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status      TEXT        NOT NULL DEFAULT 'pending',
    currency    CHAR(3)     NOT NULL,
    total_cents BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT orders_status_check
        CHECK (status IN ('pending', 'paid', 'shipped', 'delivered', 'cancelled', 'refunded')),
    CONSTRAINT orders_total_non_negative CHECK (total_cents >= 0)
);

-- Serves "show my orders, newest first" without scanning the whole table.
CREATE INDEX orders_user_id_created_at_idx ON orders (user_id, created_at DESC);

CREATE TABLE order_items (
    id               BIGSERIAL   PRIMARY KEY,
    order_id         BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id       BIGINT      NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    -- Snapshots: copied from the product at checkout so later edits to the
    -- product's name or price never change what the customer actually bought.
    product_name     TEXT        NOT NULL,
    unit_price_cents BIGINT      NOT NULL,
    quantity         INTEGER     NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT order_items_price_non_negative CHECK (unit_price_cents >= 0),
    CONSTRAINT order_items_quantity_positive CHECK (quantity > 0)
);

CREATE INDEX order_items_order_id_idx ON order_items (order_id);
CREATE INDEX order_items_product_id_idx ON order_items (product_id);
