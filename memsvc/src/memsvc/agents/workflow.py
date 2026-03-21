"""LangGraph workflow for session processing."""

import json
import logging
from datetime import datetime
from pathlib import Path
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import HumanMessage
from langgraph.graph import StateGraph, START, END
from langgraph.checkpoint.memory import InMemorySaver

from memsvc.agents.state import SessionWorkflowState, AgentResult
from memsvc.agents.tools import create_memory_tools
from memsvc.config import settings
from memsvc.core.llm import QwenLLM, MockLLM
from memsvc.core.indexer import MemoryIndexer
from memsvc.core.metadata import MetadataManager
from memsvc.utils.prompt_loader import PromptLoader

logger = logging.getLogger(__name__)

# Try to import SqliteSaver for persistent checkpointing
# Falls back to InMemorySaver if not available
try:
    import sqlite3
    from langgraph_checkpoint_sqlite import SqliteSaver
    HAS_SQLITE_SAVER = True
except ImportError:
    SqliteSaver = None
    HAS_SQLITE_SAVER = False


class SessionWorkflow:
    """LangGraph workflow for complete session processing.

    Handles the full pipeline:
    1. Write histories
    2. Compress to notes
    3. Run memory agents in parallel
    4. Trigger re-embed
    """

    def __init__(
        self,
        llm: QwenLLM | MockLLM,
        memory_dir: Path | None = None,
        metadata_manager: MetadataManager | None = None,
        indexer: MemoryIndexer | None = None,
    ):
        self.llm = llm
        self.memory_dir = memory_dir or settings.memory_dir
        self.metadata_manager = metadata_manager
        self.indexer = indexer
        self.prompt_loader = PromptLoader(settings.effective_prompts_dir)

        # Directories
        self.histories_dir = self.memory_dir / "histories"
        self.notes_dir = self.memory_dir / "notes"

        # Ensure directories exist
        self.histories_dir.mkdir(parents=True, exist_ok=True)
        self.notes_dir.mkdir(parents=True, exist_ok=True)

        # Graph and checkpointer
        self._graph = None
        self._checkpointer = None

    def _get_checkpointer(self):
        """Get or create checkpointer.

        Uses SqliteSaver for persistent checkpointing if available,
        otherwise falls back to InMemorySaver.
        """
        if self._checkpointer is None:
            if HAS_SQLITE_SAVER:
                db_path = settings.data_dir / "session_workflow.db"
                conn = sqlite3.connect(str(db_path), check_same_thread=False)
                self._checkpointer = SqliteSaver(conn)
                logger.info("Using SqliteSaver for workflow checkpointing")
            else:
                self._checkpointer = InMemorySaver()
                logger.info("Using InMemorySaver for workflow checkpointing (sqlite not available)")
        return self._checkpointer

    # === Node Functions ===

    async def write_histories_node(self, state: SessionWorkflowState) -> dict:
        """Write messages to histories file."""
        session_id = state["session_id"]
        messages = state["messages"]

        histories_path = self.histories_dir / f"{session_id}.json"

        # Skip if already exists (for retry scenarios)
        if histories_path.exists():
            logger.info(f"Histories already exists for {session_id}, skipping")
            return {
                "histories_written": True,
                "histories_path": str(histories_path),
            }

        session_data = {
            "session_id": session_id,
            "messages": messages,
            "created_at": datetime.now().isoformat(),
        }

        histories_path.write_text(
            json.dumps(session_data, indent=2, ensure_ascii=False),
            encoding="utf-8"
        )

        logger.info(f"Wrote histories for {session_id}: {len(messages)} messages")
        return {
            "histories_written": True,
            "histories_path": str(histories_path),
        }

    async def compress_notes_node(self, state: SessionWorkflowState) -> dict:
        """Compress messages to notes using LLM."""
        session_id = state["session_id"]
        messages = state["messages"]

        notes_path = self.notes_dir / f"{session_id}.md"

        # Skip if already exists
        if notes_path.exists():
            logger.info(f"Notes already exists for {session_id}, skipping")
            content = notes_path.read_text(encoding="utf-8")
            return {
                "notes_written": True,
                "notes_path": str(notes_path),
                "notes_content": content,
            }

        # Load compression prompt
        prompt = self.prompt_loader.load(
            "session_compress",
            session_id=session_id,
            message_count=len(messages),
            messages=messages,
        )

        # Call LLM
        logger.info(f"Compressing notes for {session_id}")
        compressed = await self.llm.complete(prompt)

        # Generate description from first meaningful line
        lines = compressed.strip().split("\n")
        description = ""
        for line in lines:
            line = line.strip()
            if line and not line.startswith("#"):
                description = line[:200]
                break
        if not description:
            description = f"Session {session_id} summary"

        # Create frontmatter with source_history reference
        created_at = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
        histories_filename = f"{session_id}.json"
        frontmatter = f"""---
description: {description}
created_at: {created_at}
source_history: histories/{histories_filename}
---

"""
        full_content = frontmatter + compressed
        notes_path.write_text(full_content, encoding="utf-8")

        logger.info(f"Wrote notes for {session_id}")
        return {
            "notes_written": True,
            "notes_path": str(notes_path),
            "notes_content": full_content,
        }

    async def load_agent_context_node(self, state: SessionWorkflowState) -> dict:
        """Load existing memories for agent context."""
        return {
            "current_self": self._read_file("inner/self.md"),
            "current_user": self._read_file("inner/user.md"),
        }

    async def experience_agent_node(self, state: SessionWorkflowState) -> dict:
        """Run experience extraction agent."""
        if not settings.agent_enabled:
            logger.info("Experience agent disabled, skipping")
            return {"agent_results": []}

        return await self._run_agent(
            agent_name="experience",
            write_dir="experience",
            prompt_template="experience_agent",
            state=state,
        )

    async def knowledge_agent_node(self, state: SessionWorkflowState) -> dict:
        """Run knowledge extraction agent."""
        if not settings.agent_enabled:
            logger.info("Knowledge agent disabled, skipping")
            return {"agent_results": []}

        return await self._run_agent(
            agent_name="knowledge",
            write_dir="knowledge",
            prompt_template="knowledge_agent",
            state=state,
        )

    async def inner_agent_node(self, state: SessionWorkflowState) -> dict:
        """Run inner/self/user agent."""
        if not settings.agent_enabled:
            logger.info("Inner agent disabled, skipping")
            return {"agent_results": []}

        return await self._run_agent(
            agent_name="inner",
            write_dir="inner",
            prompt_template="inner_agent",
            state=state,
        )

    async def re_embed_node(self, state: SessionWorkflowState) -> dict:
        """Trigger re-indexing of memory files."""
        if not self.indexer or not self.metadata_manager:
            logger.warning("Indexer not available, skipping re_embed")
            return {"re_embed_done": True, "files_indexed": []}

        try:
            # Scan for changes
            changes = await self.metadata_manager.scan_and_update()

            # Reindex pending files
            results = await self.indexer.reindex_all()

            indexed = [r.path for r in results if r.success]
            logger.info(f"Re-embedded {len(indexed)} files")

            return {
                "re_embed_done": True,
                "files_indexed": indexed,
            }
        except Exception as e:
            logger.error(f"Re-embed failed: {e}")
            return {
                "re_embed_done": False,
                "files_indexed": [],
            }

    # === Helper Methods ===

    def _read_directory_summary(self, subdir: str, limit: int = 5) -> str:
        """Read summary of files from a directory."""
        dir_path = self.memory_dir / subdir
        if not dir_path.exists():
            return ""

        contents = []
        for md_file in sorted(dir_path.glob("*.md"))[:limit]:
            try:
                content = md_file.read_text(encoding="utf-8")
                # Truncate long files
                if len(content) > 2000:
                    content = content[:2000] + "\n... (truncated)"
                contents.append(f"--- {md_file.name} ---\n{content}")
            except Exception:
                continue

        return "\n\n".join(contents)

    def _read_file(self, path: str) -> str:
        """Read a single file."""
        full_path = self.memory_dir / path
        if full_path.exists():
            try:
                return full_path.read_text(encoding="utf-8")
            except Exception:
                return ""
        return ""

    async def _run_agent(
        self,
        agent_name: str,
        write_dir: str,
        prompt_template: str,
        state: SessionWorkflowState,
    ) -> dict:
        """Run a memory agent using LangChain's create_agent.

        The agent has access to file tools (read, write, edit, list, grep)
        and will use them to analyze notes and update memory files.
        """
        try:
            # Load the system prompt template
            prompt = self.prompt_loader.load(
                prompt_template,
                notes_content=state["notes_content"],
                current_self=state.get("current_self", ""),
                current_user=state.get("current_user", ""),
            )

            # Create tools for this agent (sandboxed to write_dir)
            tools = create_memory_tools(write_dir, self.memory_dir)

            # Skip agent execution if using mock LLM (no tool calling support)
            if isinstance(self.llm, MockLLM):
                logger.info(f"{agent_name} agent: mock LLM, simulating execution")
                return {
                    "agent_results": [{
                        "agent_name": agent_name,
                        "success": True,
                        "files_written": [],
                        "files_edited": [],
                        "error": None,
                    }]
                }

            # Create agent with tools using LangChain's create_agent
            agent = create_agent(
                model=self.llm.chat_model,
                tools=tools,
                system_prompt=prompt,
                name=agent_name,
            )

            # Execute agent
            logger.info(f"Running {agent_name} agent for {state['session_id']}")
            result = await agent.ainvoke({
                "messages": [
                    HumanMessage(content="Analyze the session notes and update memories as needed.")
                ]
            })

            logger.info(f"{agent_name} agent completed for {state['session_id']}")

            return {
                "agent_results": [{
                    "agent_name": agent_name,
                    "success": True,
                    "files_written": [],
                    "files_edited": [],
                    "error": None,
                }]
            }

        except Exception as e:
            logger.error(f"{agent_name} agent failed: {e}")
            return {
                "agent_results": [{
                    "agent_name": agent_name,
                    "success": False,
                    "files_written": [],
                    "files_edited": [],
                    "error": str(e),
                }]
            }

    # === Graph Building ===

    def build_graph(self) -> StateGraph:
        """Build the complete workflow graph."""
        builder = StateGraph(SessionWorkflowState)

        # Add nodes
        builder.add_node("write_histories", self.write_histories_node)
        builder.add_node("compress_notes", self.compress_notes_node)
        builder.add_node("load_agent_context", self.load_agent_context_node)
        builder.add_node("experience_agent", self.experience_agent_node)
        builder.add_node("knowledge_agent", self.knowledge_agent_node)
        builder.add_node("inner_agent", self.inner_agent_node)
        builder.add_node("re_embed", self.re_embed_node)

        # Define flow
        builder.add_edge(START, "write_histories")
        builder.add_edge("write_histories", "compress_notes")
        builder.add_edge("compress_notes", "load_agent_context")

        # Parallel agents
        builder.add_edge("load_agent_context", "experience_agent")
        builder.add_edge("load_agent_context", "knowledge_agent")
        builder.add_edge("load_agent_context", "inner_agent")

        # Converge to re_embed
        builder.add_edge("experience_agent", "re_embed")
        builder.add_edge("knowledge_agent", "re_embed")
        builder.add_edge("inner_agent", "re_embed")

        builder.add_edge("re_embed", END)

        return builder.compile(checkpointer=self._get_checkpointer())

    async def run(
        self,
        session_id: str,
        messages: list[dict],
    ) -> SessionWorkflowState:
        """Execute the complete session workflow.

        Args:
            session_id: Unique session identifier
            messages: List of message dicts with 'role' and 'content'

        Returns:
            Final workflow state
        """
        if self._graph is None:
            self._graph = self.build_graph()

        initial_state: SessionWorkflowState = {
            "session_id": session_id,
            "messages": messages,
            "histories_written": False,
            "histories_path": None,
            "notes_written": False,
            "notes_path": None,
            "notes_content": "",
            "current_self": "",
            "current_user": "",
            "agent_results": [],
            "re_embed_done": False,
            "files_indexed": [],
            "started_at": datetime.now(),
            "completed_at": None,
            "error": None,
            "retry_count": 0,
        }

        config = {"configurable": {"thread_id": session_id}}

        try:
            result = await self._graph.ainvoke(initial_state, config)
            result["completed_at"] = datetime.now()
            return result
        except Exception as e:
            logger.error(f"Workflow failed for {session_id}: {e}")
            return {
                **initial_state,
                "error": str(e),
                "completed_at": datetime.now(),
            }