#!/bin/bash
mongosh --eval "
  db = db.getSiblingDB('wifi');
  db.createUser({
    user: '$MONGO_WIFI_USER',
    pwd: '$MONGO_WIFI_PASSWORD',
    roles: [
      { role: 'readWrite', db: 'wifi' }
    ]
  });
"
