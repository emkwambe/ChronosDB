/**
 * ChronosDB JavaScript Client Library
 */

class ChronosDB {
    constructor(baseUrl = "http://localhost:8080", database = "test", apiKey = null) {
        this.baseUrl = baseUrl;
        this.database = database;
        this.apiKey = apiKey;
    }

    _url(endpoint) {
        return `${this.baseUrl}/v1/db/${this.database}/${endpoint}`;
    }

    _headers() {
        const headers = { "Content-Type": "application/json" };
        if (this.apiKey) {
            headers["X-API-Key"] = this.apiKey;
        }
        return headers;
    }

    async health() {
        const response = await fetch(this._url("health"));
        return response.json();
    }

    async query(queryStr) {
        const response = await fetch(this._url("query"), {
            method: "POST",
            headers: this._headers(),
            body: JSON.stringify({ query: queryStr })
        });
        const data = await response.json();
        return data.results || [];
    }

    async createNode(label, properties) {
        const propsStr = Object.entries(properties)
            .map(([k, v]) => typeof v === "string" ? `${k}: '${v}'` : `${k}: ${v}`)
            .join(", ");
        const query = `CREATE (n:${label} {${propsStr}})`;
        const results = await this.query(query);
        return results[0];
    }

    async matchNodes(label, limit = 100) {
        const query = `MATCH (n:${label}) RETURN n LIMIT ${limit}`;
        return this.query(query);
    }

    async matchWithFilter(label, field, operator, value) {
        const query = `MATCH (n:${label}) WHERE n.${field} ${operator} ${value} RETURN n`;
        return this.query(query);
    }

    async timeTravel(label, timestamp) {
        const query = `MATCH (n:${label}) RETURN n AS OF ${timestamp}`;
        return this.query(query);
    }

    async forecast(nodeId, property, days = 30) {
        const query = `FORECAST ${property} OVER ${days} DAYS FOR ${nodeId}`;
        const results = await this.query(query);
        return results[0];
    }

    async deleteNode(label, field, value) {
        const query = `DELETE (n:${label}) WHERE n.${field} = '${value}'`;
        await this.query(query);
        return true;
    }
}

// Example usage
async function example() {
    const db = new ChronosDB();
    
    // Health check
    const health = await db.health();
    console.log("Health:", health);
    
    // Create node
    const node = await db.createNode("Person", { name: "Alice", age: 30 });
    console.log("Created:", node);
    
    // Query nodes
    const nodes = await db.matchNodes("Person");
    console.log("Nodes:", nodes);
}

// Export for Node.js
if (typeof module !== "undefined" && module.exports) {
    module.exports = ChronosDB;
}
