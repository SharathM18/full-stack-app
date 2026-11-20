-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    username VARCHAR(50) UNIQUE NOT NULL CONSTRAINT username_length CHECK (char_length(username) >= 3),
    email VARCHAR(255) UNIQUE NOT NULL
);

CREATE UNIQUE INDEX idx_users_username ON users (LOWER(username));
CREATE UNIQUE INDEX idx_users_email ON users (LOWER(email));

CREATE TRIGGER set_timestamp_users
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- Products table
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    title VARCHAR(255) NOT NULL,
    description TEXT,
    price INT NOT NULL CONSTRAINT positive_price CHECK (price > 0),
    stock_quantity INT NOT NULL DEFAULT 0 CONSTRAINT valid_stock CHECK (stock_quantity >= 0),
    size TEXT NOT NULL CONSTRAINT valid_size CHECK (size IN ('small', 'medium', 'large')),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}'::jsonb,

    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_products_user_id ON products(user_id);
CREATE INDEX idx_products_status_price ON products(status, price) WHERE status = 'active';
CREATE INDEX idx_products_price_status ON products(price) WHERE status = 'active';

CREATE TRIGGER set_timestamp_products
BEFORE UPDATE ON products
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- Categories table
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    name VARCHAR(50) NOT NULL,
    description TEXT,

    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_categories_user_id ON categories(user_id);

CREATE TRIGGER set_timestamp_categories
BEFORE UPDATE ON categories
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- Junction table for Product <-> Category (Many-to-many)
CREATE TABLE product_categories (
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, category_id)
);

CREATE INDEX idx_product_categories_category_id ON product_categories(category_id);
