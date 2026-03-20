"""LLM providers for session compression and agents."""

import asyncio
import logging

from langchain_community.chat_models.tongyi import ChatTongyi
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import HumanMessage, SystemMessage

from memsvc.config import settings

logger = logging.getLogger(__name__)


class QwenLLM:
    """Qwen ChatModel via LangChain ChatTongyi.

    Provides both:
    - chat_model: LangChain ChatModel for agents with tool calling
    - complete(): Backward-compatible method for session compression
    """

    def __init__(
        self,
        api_key: str | None = None,
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ):
        """Initialize Qwen LLM provider via LangChain.

        Args:
            api_key: DashScope API key. Defaults to settings.effective_llm_api_key.
            model: Model name. Defaults to settings.llm_model.
            max_tokens: Max tokens. Defaults to settings.llm_max_tokens.
            temperature: Temperature. Defaults to settings.llm_temperature.
        """
        self._api_key = api_key or settings.effective_llm_api_key
        self._model = model or settings.llm_model
        self._max_tokens = max_tokens or settings.llm_max_tokens
        self._temperature = temperature if temperature is not None else settings.llm_temperature

        if not self._api_key:
            raise ValueError("Qwen LLM requires an API key. Set MEMSVC_LLM_API_KEY or MEMSVC_EMBEDDING_API_KEY.")

        # Create LangChain ChatModel with tool calling support via ChatTongyi
        self.chat_model: BaseChatModel = ChatTongyi(
            model=self._model,
            api_key=self._api_key,
            temperature=self._temperature,
            max_tokens=self._max_tokens,
        )

        logger.info(f"Initialized Qwen ChatTongyi: {self._model}")

    @property
    def model(self) -> str:
        return self._model

    async def complete(
        self,
        prompt: str,
        system: str | None = None,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> str:
        """Generate completion using LangChain ChatTongyi.

        Backward-compatible method for session compression.

        Args:
            prompt: The user prompt.
            system: Optional system prompt.
            max_tokens: Maximum tokens to generate.
            temperature: Sampling temperature.

        Returns:
            Generated text.

        Raises:
            RuntimeError: If API call fails.
        """
        messages = []
        if system:
            messages.append(SystemMessage(content=system))
        messages.append(HumanMessage(content=prompt))

        config = {}
        if max_tokens:
            config["max_tokens"] = max_tokens
        if temperature is not None:
            config["temperature"] = temperature

        try:
            response = await self.chat_model.ainvoke(messages, config=config)
            content = response.content
            logger.debug(f"LLM completion generated: {len(content)} chars")
            return content
        except Exception as e:
            error_msg = f"LangChain ChatTongyi error: {e}"
            logger.error(error_msg)
            raise RuntimeError(error_msg)


class MockLLM:
    """Mock LLM provider for testing without external dependencies.

    Returns a fixed response pattern useful for testing.
    Provides a mock chat_model attribute for agent testing.
    """

    def __init__(self, model: str = "mock-llm"):
        """Initialize mock LLM provider.

        Args:
            model: Model name for identification.
        """
        self._model = model
        self.chat_model = None  # No chat model for mock

    @property
    def model(self) -> str:
        return self._model

    async def complete(
        self,
        prompt: str,
        system: str | None = None,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> str:
        """Generate mock completion.

        Returns a structured mock response for testing.

        Args:
            prompt: The user prompt (used to generate deterministic content).
            system: Optional system prompt (ignored).
            max_tokens: Maximum tokens (ignored).
            temperature: Temperature (ignored).

        Returns:
            Mock completion text.
        """
        # Simulate async behavior
        await asyncio.sleep(0.01)

        # Generate a mock summary based on the prompt
        mock_response = """## Overview
This is a mock session summary for testing purposes.

## Topics Discussed
- Topic 1: Testing the session compression
- Topic 2: Mock LLM provider

## Key Decisions
- Use mock provider for testing

## Actions Taken
- Created test session
- Verified compression pipeline

## Important Context
- This is a test session with mock data
"""
        logger.debug(f"Mock LLM completion: {len(mock_response)} chars")
        return mock_response


def get_llm_provider(provider: str | None = None) -> QwenLLM | MockLLM:
    """Factory function to create LLM provider.

    Args:
        provider: Provider name ("qwen" or "mock"). Defaults to settings.llm_provider.

    Returns:
        LLM instance (QwenLLM or MockLLM).

    Raises:
        ValueError: If provider name is unknown.
    """
    provider = provider or settings.llm_provider

    if provider == "qwen":
        return QwenLLM()
    elif provider == "mock":
        return MockLLM()
    else:
        raise ValueError(f"Unknown LLM provider: {provider}")
