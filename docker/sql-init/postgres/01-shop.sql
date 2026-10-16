-- Sample relational source for migration tests (PostgreSQL).
CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email VARCHAR(200), vip BOOLEAN, balance NUMERIC(10,2), joined TIMESTAMP, prefs JSON, avatar BYTEA);
CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER, total NUMERIC(12,2), placed_at DATE, note TEXT);
CREATE TABLE order_lines (order_id INTEGER REFERENCES orders(id), line INTEGER, sku TEXT, qty INTEGER, PRIMARY KEY (order_id, line));
CREATE TABLE logs (msg TEXT, level REAL);
INSERT INTO customers VALUES (1, 'Ada', 'ada@example.com', true, 1234.50, '2024-01-31 10:00:00', '{"theme":"dark","tags":["a","b"]}', '\x00ff10');
INSERT INTO customers VALUES (2, 'Grace', NULL, false, 0.10, '2023-06-01 08:30:00', NULL, NULL);
INSERT INTO customers VALUES (3, 'Linus', 'linus@example.com', NULL, 99999999.99, NULL, '{}', NULL);
INSERT INTO orders VALUES (10, 1, 19.99, '2024-02-01', 'first'), (11, 1, 5.00, '2024-02-02', NULL), (12, 2, 100.00, '2024-03-01', 'bulk'), (13, 99, 1.00, '2024-03-02', 'orphan');
-- NOT VALID keeps the deliberate orphan row (order 13) while still declaring the relationship.
ALTER TABLE orders ADD CONSTRAINT orders_customer_fk FOREIGN KEY (customer_id) REFERENCES customers(id) NOT VALID;
INSERT INTO order_lines VALUES (10, 1, 'KB-001', 1), (10, 2, 'MS-002', 2), (12, 1, 'HD-004', 10);
INSERT INTO logs VALUES ('boot', 1.5), ('ready', NULL);
-- Least privilege: the migration only needs to read.
CREATE ROLE mango_reader LOGIN PASSWORD 'reader';
GRANT SELECT ON ALL TABLES IN SCHEMA public TO mango_reader;
