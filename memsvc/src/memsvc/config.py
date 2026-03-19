"""Configuration management for memory service."""

from pathlib import Path
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Memory service configuration."""

    model_config = SettingsConfigDict(
        env_prefix="MEMSVC_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    # Service configuration
    host: str = "127.0.0.1"
    port: int = 8765
    log_level: Literal["debug", "info", "warning", "error"] = "info"

    # Path configuration
    memory_dir: Path = Field(
        default_factory=lambda: Path.home() / ".local" / "share" / "oxencode" / "memory"
    )
    data_dir: Path = Field(
        default_factory=lambda: Path.home() / ".local" / "share" / "oxencode"
    )

    # Watch configuration
    watch_enabled: bool = True
    watch_dirs: list[str] = ["experience", "knowledge", "notes"]

    # File constraints
    max_file_size: int = 100 * 1024  # 100KB max per file
    supported_extensions: list[str] = [".md", ".json", ".txt"]

    # Embedding configuration
    embedding_provider: Literal["qwen", "mock"] = "qwen"
    embedding_model: str = "text-embedding-v3"
    embedding_api_key: str = "sk-8440a58764c846c88183bfec2d94d279"
    embedding_dimension: int = 1024  # text-embedding-v3 dimension

    # Chunking configuration
    chunk_size: int = 500
    chunk_overlap: int = 50
    chunk_separators: list[str] = ["\n\n", "\n", "。", "，", " ", ""]

    @property
    def db_path(self) -> Path:
        """Path to metadata database."""
        return self.data_dir / "metadata.db"

    @property
    def chroma_persist_dir(self) -> Path:
        """Path to ChromaDB persistence directory."""
        return self.data_dir / "chromadb"

    def ensure_directories(self) -> None:
        """Create necessary directories."""
        self.data_dir.mkdir(parents=True, exist_ok=True)
        self.memory_dir.mkdir(parents=True, exist_ok=True)

        # Create memory subdirectories
        for subdir in ["experience", "knowledge", "notes", "histories", "inner"]:
            (self.memory_dir / subdir).mkdir(parents=True, exist_ok=True)

        # Create chromadb directory
        self.chroma_persist_dir.mkdir(parents=True, exist_ok=True)


# Global settings instance
settings = Settings()