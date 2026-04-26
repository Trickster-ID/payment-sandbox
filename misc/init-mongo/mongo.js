const databaseName = "payment_sandbox";
const collections = ["journey_logs"];

const targetDB = db.getSiblingDB(databaseName);
const existingCollections = targetDB.getCollectionNames();

for (const name of collections) {
  if (!existingCollections.includes(name)) {
    targetDB.createCollection(name);
    print(`Created collection: ${databaseName}.${name}`);
  } else {
    print(`Collection already exists: ${databaseName}.${name}`);
  }
}

targetDB.journey_logs.createIndex({ journey_id: 1, occurred_at: 1 });
targetDB.journey_logs.createIndex({ entity_type: 1, entity_id: 1, occurred_at: -1 });

print(`MongoDB init complete for database: ${databaseName}`);
