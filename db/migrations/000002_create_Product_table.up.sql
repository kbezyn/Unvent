CREATE TABLE IF NOT EXISTS Products
(
    ID SERIAL PRIMARY KEY,
    Name VARCHAR(255) NOT NULL,
    Description TEXT,
    Features JSON,
    Weight FLOAT,
    Barcode VARCHAR(100)
);
