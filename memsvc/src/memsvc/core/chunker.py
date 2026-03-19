"""Text chunking utilities using LangChain."""

from langchain_text_splitters import RecursiveCharacterTextSplitter

from memsvc.config import settings


def get_text_splitter(
    chunk_size: int | None = None,
    chunk_overlap: int | None = None,
    separators: list[str] | None = None,
) -> RecursiveCharacterTextSplitter:
    """Get text splitter configured for memory files.

    Args:
        chunk_size: Size of each chunk. Defaults to settings.chunk_size.
        chunk_overlap: Overlap between chunks. Defaults to settings.chunk_overlap.
        separators: List of separators in priority order. Defaults to settings.chunk_separators.

    Returns:
        Configured RecursiveCharacterTextSplitter instance.
    """
    return RecursiveCharacterTextSplitter(
        separators=separators or settings.chunk_separators,
        chunk_size=chunk_size or settings.chunk_size,
        chunk_overlap=chunk_overlap or settings.chunk_overlap,
    )


def chunk_text(
    text: str,
    chunk_size: int | None = None,
    chunk_overlap: int | None = None,
) -> list[str]:
    """Split text into chunks for embedding.

    Args:
        text: The text to split.
        chunk_size: Size of each chunk. Defaults to settings.chunk_size.
        chunk_overlap: Overlap between chunks. Defaults to settings.chunk_overlap.

    Returns:
        List of text chunks.
    """
    splitter = get_text_splitter(chunk_size, chunk_overlap)
    return splitter.split_text(text)