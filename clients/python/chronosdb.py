"""
ChronosDB Python Client Library
"""

import requests
import json
from typing import Dict, List, Any, Optional

class ChronosDB:
    """ChronosDB Python Client"""
    
    def __init__(self, base_url: str = "http://localhost:8080", database: str = "test", api_key: Optional[str] = None):
        self.base_url = base_url
        self.database = database
        self.api_key = api_key
        self.session = requests.Session()
        
        if api_key:
            self.session.headers.update({"X-API-Key": api_key})
    
    def _url(self, endpoint: str) -> str:
        return f"{self.base_url}/v1/db/{self.database}/{endpoint}"
    
    def health(self) -> Dict:
        """Check database health"""
        response = self.session.get(self._url("health"))
        response.raise_for_status()
        return response.json()
    
    def query(self, query_str: str) -> List[Dict]:
        """Execute a ChronoSQL query"""
        response = self.session.post(
            self._url("query"),
            json={"query": query_str}
        )
        response.raise_for_status()
        data = response.json()
        return data.get("results", [])
    
    def create_node(self, label: str, properties: Dict) -> Dict:
        """Create a node"""
        props_str = ", ".join([f"{k}: '{v}'" if isinstance(v, str) else f"{k}: {v}" 
                               for k, v in properties.items()])
        query = f"CREATE (n:{label} {{{props_str}}})"
        results = self.query(query)
        return results[0] if results else {}
    
    def match_nodes(self, label: str, limit: int = 100) -> List[Dict]:
        """Match nodes by label"""
        query = f"MATCH (n:{label}) RETURN n LIMIT {limit}"
        return self.query(query)
    
    def match_with_filter(self, label: str, field: str, operator: str, value) -> List[Dict]:
        """Match nodes with filter"""
        query = f"MATCH (n:{label}) WHERE n.{field} {operator} {value} RETURN n"
        return self.query(query)
    
    def time_travel(self, label: str, timestamp: int) -> List[Dict]:
        """Time travel query"""
        query = f"MATCH (n:{label}) RETURN n AS OF {timestamp}"
        return self.query(query)
    
    def forecast(self, node_id: str, property: str, days: int = 30) -> Dict:
        """Forecast future values"""
        query = f"FORECAST {property} OVER {days} DAYS FOR {node_id}"
        results = self.query(query)
        return results[0] if results else {}
    
    def delete_node(self, label: str, field: str, value) -> bool:
        """Soft delete a node"""
        query = f"DELETE (n:{label}) WHERE n.{field} = '{value}'"
        try:
            self.query(query)
            return True
        except:
            return False

# Example usage
if __name__ == "__main__":
    db = ChronosDB()
    
    # Health check
    print("Health:", db.health())
    
    # Create a node
    result = db.create_node("Person", {"name": "Alice", "age": 30, "city": "New York"})
    print("Created:", result)
    
    # Query nodes
    nodes = db.match_nodes("Person")
    print("Nodes:", nodes)
    
    # Filter query
    adults = db.match_with_filter("Person", "age", ">", 25)
    print("Adults:", adults)
    
    # Forecast
    forecast = db.forecast("person_1", "age", 30)
    print("Forecast:", forecast)
