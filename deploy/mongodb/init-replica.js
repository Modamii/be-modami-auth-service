// MongoDB Replica Set Initialization Script
// This script will be executed when MongoDB container starts

print("Starting replica set initialization...");

// Wait for MongoDB to be ready
sleep(2000);

try {
    // Check if replica set is already initialized
    const status = rs.status();
    print("Replica set already initialized:", status.ok);
} catch (error) {
    print("Initializing replica set...");

    // Initialize single-node replica set
    const result = rs.initiate({
        _id: "rs0",
        members: [
            {
                _id: 0,
                host: "mongo:27017"
            }
        ]
    });

    print("Replica set initialization result:", result.ok);

    if (result.ok) {
        print("MongoDB replica set 'rs0' initialized successfully!");
        print("Connection string: mongodb://mongo1:27017/?replicaSet=rs0");
    } else {
        print("Failed to initialize replica set:", result);
    }
}

// Wait for replica set to be ready
sleep(3000);

// Verify replica set status
try {
    const finalStatus = rs.status();
    print("Final replica set status:", finalStatus.myState);

    if (finalStatus.myState === 1) {
        print("Replica set is PRIMARY and ready for CDC!");
    }
} catch (error) {
    print("Warning: Could not verify final status:", error);
}

print("Replica set initialization script completed."); 