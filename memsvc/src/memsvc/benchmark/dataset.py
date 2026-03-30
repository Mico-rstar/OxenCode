"""Dataset management for RAG benchmark.

Handles loading and creating test scenarios with memories and queries.
"""

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class MemoryEntry:
    """A memory entry for benchmarking."""

    id: str
    description: str  # Used for trigger/indexing
    content: str  # Full content
    category: str = "knowledge"  # experience, knowledge, notes
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class QueryEntry:
    """A query with expected results."""

    id: str
    text: str
    expected_ids: list[str]  # Ground truth relevant memory IDs
    expected_category: str | None = None
    query_type: str = "semantic"  # semantic, exact, paraphrase


@dataclass
class BenchmarkScenario:
    """A complete benchmark scenario."""

    id: str
    name: str
    description: str = ""
    memories: list[MemoryEntry] = field(default_factory=list)
    queries: list[QueryEntry] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)

    # Scenario-level metadata
    noise_memories: int = 0  # Number of irrelevant memories to add
    difficulty: str = "medium"  # easy, medium, hard


class ScenarioLoader:
    """Load benchmark scenarios from JSON files."""

    def __init__(self, scenarios_dir: Path | None = None):
        """Initialize scenario loader.

        Args:
            scenarios_dir: Directory containing scenario JSON files.
                         Defaults to package's scenarios/ directory.
        """
        if scenarios_dir is None:
            # Default to package's scenarios directory
            import memsvc.benchmark
            package_dir = Path(memsvc.benchmark.__file__).parent
            scenarios_dir = package_dir / "scenarios"

        self.scenarios_dir = scenarios_dir

    def load_scenario(self, scenario_id: str) -> BenchmarkScenario:
        """Load a scenario by ID.

        Args:
            scenario_id: Scenario identifier (without .json extension).

        Returns:
            BenchmarkScenario object.

        Raises:
            FileNotFoundError: If scenario file doesn't exist.
            ValueError: If scenario file is invalid.
        """
        # Try to find the file
        scenario_file = self.scenarios_dir / f"{scenario_id}.json"

        if not scenario_file.exists():
            # Try with underscores
            scenario_file = self.scenarios_dir / f"{scenario_id.replace('-', '_')}.json"

        if not scenario_file.exists():
            raise FileNotFoundError(f"Scenario not found: {scenario_id}")

        return self.load_scenario_from_file(scenario_file)

    def load_scenario_from_file(self, path: Path) -> BenchmarkScenario:
        """Load a scenario from a JSON file.

        Args:
            path: Path to the scenario JSON file.

        Returns:
            BenchmarkScenario object.
        """
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)

        # Parse memories
        memories = []
        for mem_data in data.get("memories", []):
            memories.append(MemoryEntry(
                id=mem_data["id"],
                description=mem_data["description"],
                content=mem_data.get("content", mem_data["description"]),
                category=mem_data.get("category", "knowledge"),
                metadata=mem_data.get("metadata", {}),
            ))

        # Parse queries
        queries = []
        for q_data in data.get("queries", []):
            queries.append(QueryEntry(
                id=q_data["id"],
                text=q_data["text"],
                expected_ids=q_data.get("expected_ids", []),
                expected_category=q_data.get("expected_category"),
                query_type=q_data.get("query_type", "semantic"),
            ))

        return BenchmarkScenario(
            id=data["id"],
            name=data["name"],
            description=data.get("description", ""),
            memories=memories,
            queries=queries,
            tags=data.get("tags", []),
            noise_memories=data.get("noise_memories", 0),
            difficulty=data.get("difficulty", "medium"),
        )

    def list_scenarios(self) -> list[dict[str, Any]]:
        """List all available scenarios.

        Returns:
            List of scenario summaries.
        """
        scenarios = []

        if not self.scenarios_dir.exists():
            return scenarios

        for path in self.scenarios_dir.glob("*.json"):
            try:
                scenario = self.load_scenario_from_file(path)
                scenarios.append({
                    "id": scenario.id,
                    "name": scenario.name,
                    "description": scenario.description,
                    "memory_count": len(scenario.memories),
                    "query_count": len(scenario.queries),
                    "difficulty": scenario.difficulty,
                    "tags": scenario.tags,
                })
            except Exception as e:
                # Skip invalid scenario files
                continue

        return scenarios

    def load_all_scenarios(self) -> list[BenchmarkScenario]:
        """Load all available scenarios.

        Returns:
            List of BenchmarkScenario objects.
        """
        scenarios = []

        if not self.scenarios_dir.exists():
            return scenarios

        for path in self.scenarios_dir.glob("*.json"):
            try:
                scenarios.append(self.load_scenario_from_file(path))
            except Exception:
                continue

        return scenarios


class NoiseGenerator:
    """Generate noise memories for testing robustness."""

    # Predefined noise memory templates
    NOISE_TEMPLATES = [
        {
            "id_prefix": "noise_cooking",
            "description": "如何制作{dish}的食谱",
            "content": "制作{dish}需要准备以下材料...",
            "category": "knowledge",
        },
        {
            "id_prefix": "noise_travel",
            "description": "{city}旅游攻略和景点推荐",
            "content": "{city}是一个著名的旅游城市...",
            "category": "knowledge",
        },
        {
            "id_prefix": "noise_sports",
            "description": "{sport}运动的基本规则和技巧",
            "content": "{sport}是一项受欢迎的运动...",
            "category": "knowledge",
        },
    ]

    # Fill values for templates
    DISHES = ["红烧肉", "宫保鸡丁", "麻婆豆腐", "糖醋排骨", "清蒸鱼"]
    CITIES = ["北京", "上海", "广州", "深圳", "杭州"]
    SPORTS = ["篮球", "足球", "网球", "游泳", "跑步"]

    def generate_noise_memories(self, count: int) -> list[MemoryEntry]:
        """Generate noise memories.

        Args:
            count: Number of noise memories to generate.

        Returns:
            List of MemoryEntry objects.
        """
        import random

        memories = []
        template_indices = list(range(len(self.NOISE_TEMPLATES)))
        random.shuffle(template_indices)

        for i in range(count):
            template = self.NOISE_TEMPLATES[template_indices[i % len(template_indices)]]

            if template["id_prefix"] == "noise_cooking":
                fill = {"dish": random.choice(self.DISHES)}
            elif template["id_prefix"] == "noise_travel":
                fill = {"city": random.choice(self.CITIES)}
            else:
                fill = {"sport": random.choice(self.SPORTS)}

            memories.append(MemoryEntry(
                id=f"{template['id_prefix']}_{i}",
                description=template["description"].format(**fill),
                content=template["content"].format(**fill),
                category=template["category"],
            ))

        return memories