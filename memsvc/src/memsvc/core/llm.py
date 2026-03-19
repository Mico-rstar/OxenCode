"""LLM providers for session compression."""

import asyncio
import logging
from abc import ABC, abstractmethod
from http import HTTPStatus

import dashscope
from dashscope import Generation

from memsvc.config import settings

logger = logging.getLogger(__name__)


class LLMProvider(ABC):
    """Abstract base class for LLM providers."""

    @abstractmethod
    async def complete(
        self,
        prompt: str,
        system: str | None = None,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ) -> str:
        """Generate completion for a prompt.

        Args:
            prompt: The user prompt.
            system: Optional system prompt.
            max_tokens: Maximum tokens to generate.
            temperature: Sampling temperature.

        Returns:
            Generated text.
        """
        pass

    @property
    @abstractmethod
    def model(self) -> str:
        """Return model name."""
        pass


class QwenLLM(LLMProvider):
    """Qwen LLM via DashScope SDK."""

    def __init__(
        self,
        api_key: str | None = None,
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float | None = None,
    ):
        """Initialize Qwen LLM provider.

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

        # Configure dashscope with API key
        dashscope.api_key = self._api_key

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
        """Generate completion using DashScope SDK.

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
            messages.append({"role": "system", "content": system})
        messages.append({"role": "user", "content": prompt})

        def _call_generation():
            response = Generation.call(
                model=self._model,
                messages=messages,
                max_tokens=max_tokens or self._max_tokens,
                temperature=temperature if temperature is not None else self._temperature,
            )
            return response

        response = await asyncio.to_thread(_call_generation)

        # Log full response for debugging
        logger.debug(f"DashScope response: status_code={response.status_code}, code={response.code}")

        if response.status_code != HTTPStatus.OK:
            error_msg = f"DashScope API error: {response.code} - {response.message}"
            logger.error(error_msg)
            raise RuntimeError(error_msg)

        # Check for valid output
        if response.output is None:
            error_msg = f"DashScope API returned no output: code={response.code}, message={response.message}"
            logger.error(error_msg)
            raise RuntimeError(error_msg)

        # Extract content from response
        try:
            content = response.output["choices"][0]["message"]["content"]
        except (KeyError, IndexError, TypeError) as e:
            error_msg = f"Unexpected DashScope response format: {response.output}, error: {e}"
            logger.error(error_msg)
            raise RuntimeError(error_msg)

        logger.debug(f"LLM completion generated: {len(content)} chars")
        return content


class MockLLM(LLMProvider):
    """Mock LLM provider for testing without external dependencies.

    Returns a fixed response pattern useful for testing.
    """

    def __init__(self, model: str = "mock-llm"):
        """Initialize mock LLM provider.

        Args:
            model: Model name for identification.
        """
        self._model = model

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


def get_llm_provider(provider: str | None = None) -> LLMProvider:
    """Factory function to create LLM provider.

    Args:
        provider: Provider name ("qwen" or "mock"). Defaults to settings.llm_provider.

    Returns:
        LLMProvider instance.

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