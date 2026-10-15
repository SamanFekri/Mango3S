// Seeds a small sample database on first container start (mongo image runs
// scripts in /docker-entrypoint-initdb.d only when the data volume is empty).
const store = db.getSiblingDB('sample_store')

const customers = [
  { _id: ObjectId(), name: 'Ada Lovelace', email: 'ada@example.com', tier: 'gold', joined: ISODate('2021-03-14T09:00:00Z'), address: { city: 'London', country: 'UK' }, tags: ['math', 'early-adopter'] },
  { _id: ObjectId(), name: 'Grace Hopper', email: 'grace@example.com', tier: 'platinum', joined: ISODate('2020-12-09T15:30:00Z'), address: { city: 'Arlington', country: 'US' }, tags: ['navy', 'compilers'] },
  { _id: ObjectId(), name: 'Alan Turing', email: 'alan@example.com', tier: 'silver', joined: ISODate('2022-06-23T11:45:00Z'), address: { city: 'Manchester', country: 'UK' }, tags: ['crypto'] },
  { _id: ObjectId(), name: 'Katherine Johnson', email: 'katherine@example.com', tier: 'gold', joined: ISODate('2023-08-26T08:15:00Z'), address: { city: 'Hampton', country: 'US' }, tags: ['orbits'] },
  { _id: ObjectId(), name: 'Linus Torvalds', email: 'linus@example.com', tier: 'bronze', joined: ISODate('2024-01-05T19:20:00Z'), address: { city: 'Portland', country: 'US' }, tags: [] },
]
store.customers.insertMany(customers)
store.customers.createIndex({ email: 1 }, { unique: true })

const products = [
  { sku: 'KB-001', name: 'Mechanical keyboard', price: NumberDecimal('129.00'), stock: 42, category: 'peripherals' },
  { sku: 'MS-002', name: 'Wireless mouse', price: NumberDecimal('49.90'), stock: 120, category: 'peripherals' },
  { sku: 'MN-003', name: '27" monitor', price: NumberDecimal('349.00'), stock: 15, category: 'displays' },
  { sku: 'HD-004', name: 'USB-C hub', price: NumberDecimal('39.50'), stock: 0, category: 'accessories' },
  { sku: 'CH-005', name: 'Desk chair', price: NumberDecimal('259.00'), stock: 8, category: 'furniture' },
]
store.products.insertMany(products)
store.products.createIndex({ sku: 1 }, { unique: true })

const statuses = ['pending', 'shipped', 'delivered', 'cancelled']
const orders = []
for (let i = 0; i < 250; i++) {
  const c = customers[i % customers.length]
  const items = []
  const n = 1 + (i % 3)
  for (let j = 0; j < n; j++) {
    const p = products[(i + j) % products.length]
    items.push({ sku: p.sku, qty: 1 + ((i + j) % 4), price: p.price })
  }
  orders.push({
    orderNo: NumberLong(100000 + i),
    customerId: c._id,
    status: statuses[i % statuses.length],
    items,
    createdAt: new Date(Date.UTC(2024, i % 12, 1 + (i % 28), i % 24)),
  })
}
store.orders.insertMany(orders)
store.orders.createIndex({ customerId: 1, createdAt: -1 })

print(`sample_store seeded: ${store.customers.countDocuments()} customers, ${store.products.countDocuments()} products, ${store.orders.countDocuments()} orders`)
