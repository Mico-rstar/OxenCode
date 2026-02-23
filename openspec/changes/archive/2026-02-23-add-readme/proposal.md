# Proposal: Add README for OxenCode

## Why

OxenCode is an innovative AI programming assistant project, but it currently lacks comprehensive documentation in the form of a README.md file. This makes it difficult for new users and contributors to quickly understand what OxenCode is, its key features, and how to get started. A well-structured README is essential for:

1. **Project Visibility**: Making the project approachable to potential users and contributors
2. **Quick Onboarding**: Providing immediate guidance on installation and first use
3. **Feature Communication**: Clearly communicating OxenCode's unique architectural advantages
4. **Professional Presentation**: Establishing credibility with polished documentation

## What Changes

Create comprehensive README documentation at the project root with both Chinese and English versions:

1. **README.md (English Version)** - Primary README in English including:
   - **Project Logo**: Visual branding at the top
   - **Project Overview**: Brief introduction to OxenCode
   - **Key Features**: Highlighting OxenCode's unique architectural advantages:
     - **Agent/Tool Environment Separation**: Agent execution environment is isolated from tool execution environment, improving security and reliability
     - **Long-Running Task Support**: Capable of handling extended tasks without interruption
     - **Innovative Context Management**: Multi-batch, user-transparent, asynchronous context compression strategy - distinct from traditional full-context compression approaches used by other projects
     - **Stream Processing**: Real-time display of AI reasoning and tool execution
     - **Permission System**: User authorization for dangerous operations
     - **Multi-Provider Support**: Anthropic, OpenAI, Qwen, DeepSeek, GLM, Google, and more
   - **Quick Start**: Step-by-step installation and first-run guide
   - **Configuration**: Overview of config.toml structure
   - **Usage Examples**: Common usage patterns
   - **Architecture Overview**: Link to detailed architecture documentation
   - **Contributing**: Guidelines for contributors
   - **License**: License information

2. **README.zh-CN.md (Chinese Version)** - Chinese translation of the README with identical structure and content, fully localized for Chinese users

## Capabilities

### New Capabilities

- **project-documentation**: Comprehensive project README documentation (English and Chinese versions) covering overview, features, installation, configuration, and usage

### Modified Capabilities

None - this change only adds documentation

## Impact

- **New Files**:
  - `/home/rene/projs/OxenCode/README.md` (English version - primary)
  - `/home/rene/projs/OxenCode/README.zh-CN.md` (Chinese version)
- **Project Presentation**: Improved first impression for both international and Chinese visitors
- **User Experience**: Faster onboarding for users in both languages
- **No Code Changes**: Documentation only, no impact on existing code functionality
