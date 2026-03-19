"""Prompt loader for loading and rendering prompt templates."""

import logging
from pathlib import Path

from jinja2 import Environment, FileSystemLoader, TemplateNotFound

logger = logging.getLogger(__name__)


class PromptLoader:
    """Load and render prompt templates from markdown files.

    Uses Jinja2 for template rendering with variable injection.
    Prompts are stored as .md files in the prompts directory.
    """

    def __init__(self, prompts_dir: Path):
        """Initialize the prompt loader.

        Args:
            prompts_dir: Directory containing prompt .md files.
        """
        self.prompts_dir = prompts_dir
        self._env = Environment(
            loader=FileSystemLoader(prompts_dir),
            autoescape=False,  # No HTML escaping for prompts
            trim_blocks=True,
            lstrip_blocks=True,
        )
        logger.debug(f"PromptLoader initialized with prompts_dir: {prompts_dir}")

    def load(self, prompt_id: str, **variables) -> str:
        """Load and render a prompt template.

        Args:
            prompt_id: Prompt file name without .md extension.
            **variables: Variables to inject into the template.

        Returns:
            Rendered prompt string.

        Raises:
            TemplateNotFound: If the prompt file doesn't exist.
        """
        template_name = f"{prompt_id}.md"
        try:
            template = self._env.get_template(template_name)
            rendered = template.render(**variables)
            logger.debug(f"Loaded and rendered prompt: {prompt_id}")
            return rendered
        except TemplateNotFound:
            logger.error(f"Prompt template not found: {template_name}")
            raise

    def load_raw(self, prompt_id: str) -> str:
        """Load a prompt template without rendering.

        Args:
            prompt_id: Prompt file name without .md extension.

        Returns:
            Raw template content as string.

        Raises:
            FileNotFoundError: If the prompt file doesn't exist.
        """
        template_path = self.prompts_dir / f"{prompt_id}.md"
        if not template_path.exists():
            logger.error(f"Prompt file not found: {template_path}")
            raise FileNotFoundError(f"Prompt not found: {prompt_id}")

        content = template_path.read_text(encoding="utf-8")
        logger.debug(f"Loaded raw prompt: {prompt_id}")
        return content

    def exists(self, prompt_id: str) -> bool:
        """Check if a prompt template exists.

        Args:
            prompt_id: Prompt file name without .md extension.

        Returns:
            True if the prompt file exists, False otherwise.
        """
        template_path = self.prompts_dir / f"{prompt_id}.md"
        return template_path.exists()

    def list_prompts(self) -> list[str]:
        """List all available prompt IDs.

        Returns:
            List of prompt IDs (file names without .md extension).
        """
        prompts = []
        if self.prompts_dir.exists():
            for file_path in self.prompts_dir.glob("*.md"):
                prompts.append(file_path.stem)
        return sorted(prompts)