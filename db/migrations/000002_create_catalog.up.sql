CREATE TABLE categories (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

-- lower(name): "Shoes" and "shoes" count as the same category.
CREATE UNIQUE INDEX categories_name_key ON categories (lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE products (
    id          BIGSERIAL   PRIMARY KEY,
    -- RESTRICT: you can't delete a category that still has products in it.
    category_id BIGINT      NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    -- Money is stored as whole cents in an integer, never as a float. 1999 = $19.99.
    price_cents BIGINT      NOT NULL,
    stock       INTEGER     NOT NULL DEFAULT 0,
    sku         TEXT        NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT products_price_non_negative CHECK (price_cents >= 0),
    -- Last line of defence against overselling: stock can never go below zero.
    CONSTRAINT products_stock_non_negative CHECK (stock >= 0)
);

CREATE UNIQUE INDEX products_sku_key ON products (sku) WHERE deleted_at IS NULL;
CREATE INDEX products_category_id_idx ON products (category_id);

CREATE TABLE product_images (
    id         BIGSERIAL   PRIMARY KEY,
    product_id BIGINT      NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    url        TEXT        NOT NULL,
    alt_text   TEXT        NOT NULL DEFAULT '',
    is_primary BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX product_images_product_id_idx ON product_images (product_id);
-- At most one primary image per product.
CREATE UNIQUE INDEX product_images_one_primary ON product_images (product_id) WHERE is_primary;
