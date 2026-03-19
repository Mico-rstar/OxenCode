"""Tests for prompt loader."""

import tempfile
from pathlib import Path

import pytest

from memsvc.utils.prompt_loader import PromptLoader


class TestPromptLoader:
    """Tests for PromptLoader."""

    @pytest.fixture
    def prompts_dir(self, tmp_path: Path) -> Path:
        """Create a temporary prompts directory with test templates."""
        prompts_dir = tmp_path / "prompts"
        prompts_dir.mkdir()

        # Create a simple test prompt
        test_prompt = prompts_dir / "test_prompt.md"
        test_prompt.write_text("""# Test Prompt

Hello, {{ name }}!

You have {{ count }} items.
""")

        # Create a prompt with complex Jinja2 features
        complex_prompt = prompts_dir / "complex_prompt.md"
        complex_prompt.write_text("""# Complex Prompt

{% for item in items %}
- {{ item }}
{% endfor %}

{% if show_summary %}
Summary: {{ summary }}
{% endif %}
""")

        return prompts_dir

    def test_load_simple_prompt(self, prompts_dir: Path):
        """Test loading and rendering a simple prompt."""
        loader = PromptLoader(prompts_dir)
        result = loader.load("test_prompt", name="World", count=5)

        assert "Hello, World!" in result
        assert "You have 5 items." in result

    def test_load_complex_prompt(self, prompts_dir: Path):
        """Test loading and rendering a prompt with loops and conditionals."""
        loader = PromptLoader(prompts_dir)
        result = loader.load(
            "complex_prompt",
            items=["apple", "banana", "cherry"],
            show_summary=True,
            summary="All fruits",
        )

        assert "- apple" in result
        assert "- banana" in result
        assert "- cherry" in result
        assert "Summary: All fruits" in result

    def test_load_prompt_with_missing_variable(self, prompts_dir: Path):
        """Test loading a prompt without providing all variables."""
        loader = PromptLoader(prompts_dir)
        # Jinja2 renders missing variables as empty strings by default
        result = loader.load("test_prompt", name="Test")

        assert "Hello, Test!" in result
        assert "You have  items." in result  # Missing count renders as empty

    def test_load_raw_prompt(self, prompts_dir: Path):
        """Test loading raw prompt without rendering."""
        loader = PromptLoader(prompts_dir)
        result = loader.load_raw("test_prompt")

        assert "{{ name }}" in result
        assert "{{ count }}" in result

    def test_load_nonexistent_prompt(self, prompts_dir: Path):
        """Test loading a prompt that doesn't exist."""
        loader = PromptLoader(prompts_dir)

        with pytest.raises(Exception):  # Jinja2 raises TemplateNotFound
            loader.load("nonexistent")

    def test_exists(self, prompts_dir: Path):
        """Test checking if a prompt exists."""
        loader = PromptLoader(prompts_dir)

        assert loader.exists("test_prompt") is True
        assert loader.exists("nonexistent") is False

    def test_list_prompts(self, prompts_dir: Path):
        """Test listing all available prompts."""
        loader = PromptLoader(prompts_dir)
        prompts = loader.list_prompts()

        assert "test_prompt" in prompts
        assert "complex_prompt" in prompts
        assert len(prompts) == 2

    def test_list_prompts_empty_dir(self, tmp_path: Path):
        """Test listing prompts in an empty directory."""
        empty_dir = tmp_path / "empty"
        empty_dir.mkdir()

        loader = PromptLoader(empty_dir)
        prompts = loader.list_prompts()

        assert prompts == []