CREATE TABLE IF NOT EXISTS analytics(
    id SERIAL PRIMARY KEY,
    product_name VARCHAR(255) NOT NULL, 
    total_sold INT NOT NULL, 
    total_sum DECIMAL(10, 2) 
);

CREATE TABLE IF NOT EXISTS warehouse_revenue (
    id SERIAL PRIMARY KEY,
    warehouse_id INT NOT NULL,
    address VARCHAR(255) NOT NULL,
    total_revenue DECIMAL(10, 2) NOT NULL
);

