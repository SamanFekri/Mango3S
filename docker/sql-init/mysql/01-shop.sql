-- Sample relational source for migration tests (MySQL / MariaDB).
SET FOREIGN_KEY_CHECKS = 0; -- allow the deliberate orphan row (order 13)
CREATE TABLE customers (id INT PRIMARY KEY, name TEXT NOT NULL, email VARCHAR(200), vip BOOLEAN, balance DECIMAL(10,2), joined DATETIME, prefs JSON, avatar BLOB);
CREATE TABLE orders (id INT PRIMARY KEY, customer_id INT, total DECIMAL(12,2), placed_at DATE, note TEXT, CONSTRAINT orders_customer_fk FOREIGN KEY (customer_id) REFERENCES customers(id));
CREATE TABLE order_lines (order_id INT, line INT, sku VARCHAR(20), qty INT, PRIMARY KEY (order_id, line), CONSTRAINT lines_order_fk FOREIGN KEY (order_id) REFERENCES orders(id));
CREATE TABLE logs (msg TEXT, level DOUBLE);
INSERT INTO customers VALUES (1, 'Ada', 'ada@example.com', 1, 1234.50, '2024-01-31 10:00:00', '{"theme":"dark","tags":["a","b"]}', X'00ff10');
INSERT INTO customers VALUES (2, 'Grace', NULL, 0, 0.10, '2023-06-01 08:30:00', NULL, NULL);
INSERT INTO customers VALUES (3, 'Linus', 'linus@example.com', NULL, 99999999.99, NULL, '{}', NULL);
INSERT INTO orders VALUES (10, 1, 19.99, '2024-02-01', 'first'), (11, 1, 5.00, '2024-02-02', NULL), (12, 2, 100.00, '2024-03-01', 'bulk'), (13, 99, 1.00, '2024-03-02', 'orphan');
INSERT INTO order_lines VALUES (10, 1, 'KB-001', 1), (10, 2, 'MS-002', 2), (12, 1, 'HD-004', 10);
INSERT INTO logs VALUES ('boot', 1.5), ('ready', NULL);
SET FOREIGN_KEY_CHECKS = 1;
-- Least privilege: the migration only needs to read.
CREATE USER 'mango_reader'@'%' IDENTIFIED BY 'reader';
GRANT SELECT ON shop.* TO 'mango_reader'@'%';
