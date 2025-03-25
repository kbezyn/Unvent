CREATE TABLE IF NOT EXISTS Inventory
(
    ID  SERIAL,
    ProductID INT NOT NULL,
    WarehouseID INT NOT NULL,
    Quantity INT NOT NULL,
    Price FLOAT NOT NULL,
    Discount FLOAT CHECK (Discount >= 0.0 AND Discount <= 1.0)
);
