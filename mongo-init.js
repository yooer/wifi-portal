db = db.getSiblingDB('wifi');
db.createUser({
  user: process.env.MONGO_WIFI_USER,
  pwd: process.env.MONGO_WIFI_PASSWORD,
  roles: [
    { role: 'readWrite', db: 'wifi' }
  ]
});
