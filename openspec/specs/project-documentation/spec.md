# Spec: Project Documentation

## Requirements

### Requirement: Project README provides comprehensive overview

The system SHALL provide a comprehensive README.md file at the project root that serves as the primary entry point for users and contributors, containing project overview, key features, installation instructions, configuration guide, usage examples, and links to detailed documentation.

#### Scenario: New user visits repository
- **WHEN** a user navigates to the OxenCode repository
- **THEN** they SHALL see README.md as the landing page
- **AND** the README SHALL display a project logo or banner
- **AND** the README SHALL contain a clear project tagline
- **AND** the README SHALL list key features with emphasis on architectural advantages

#### Scenario: User seeks installation guidance
- **WHEN** a user wants to install OxenCode
- **THEN** the README SHALL provide step-by-step installation instructions
- **AND** the README SHALL list prerequisites (Go version, dependencies)
- **AND** the README SHALL include build instructions
- **AND** the README SHALL reference config.example.toml for configuration

#### Scenario: User seeks architectural understanding
- **WHEN** a user wants to understand OxenCode's architecture
- **THEN** the README SHALL highlight three key differentiators:
  - Agent/Tool Environment Separation
  - Long-Running Task Support
  - Innovative Context Management (multi-batch, user-transparent, asynchronous)
- **AND** the README SHALL provide links to detailed documentation in docs/ directory

### Requirement: README supports bilingual documentation

The system SHALL provide both English and Chinese versions of the README, with the English version (README.md) as the primary file and the Chinese version (README.zh-CN.md) as a fully-localized translation.

#### Scenario: Chinese user visits repository
- **WHEN** a Chinese-speaking user navigates to the OxenCode repository
- **THEN** they SHALL see README.md (English version) as the default landing page
- **AND** they SHALL be able to access README.zh-CN.md for Chinese documentation
- **AND** the Chinese version SHALL contain identical structure and content as the English version
- **AND** the Chinese version SHALL use natural, localized phrasing (not machine translation)

#### Scenario: README version consistency
- **WHEN** either README.md or README.zh-CN.md is updated
- **THEN** both versions SHALL maintain identical section structure
- **AND** both versions SHALL communicate the same features and information
- **AND** code examples SHALL be identical across both versions

### Requirement: README accurately reflects project capabilities

The system SHALL ensure that all features and capabilities described in the README are currently implemented and functional in the codebase.

#### Scenario: Feature verification
- **WHEN** a feature is described in the README
- **THEN** that feature SHALL be implemented in the current codebase
- **AND** the feature description SHALL be accurate and verifiable
- **AND** the README SHALL not describe planned or roadmap features as current capabilities

#### Scenario: Documentation link validation
- **WHEN** the README includes links to detailed documentation
- **THEN** those links SHALL reference existing files in the docs/ directory
- **AND** the linked documentation SHALL exist and be accessible
- **AND** the links SHALL use relative paths for repository portability

### Requirement: README follows open-source documentation standards

The system SHALL ensure the README follows established open-source documentation conventions with appropriate sections, formatting, and presentation.

#### Scenario: README structure
- **WHEN** a user views the README
- **THEN** it SHALL contain the following sections in order:
  1. Logo/Banner
  2. Project tagline and brief description
  3. Key features
  4. Quick Start
  5. Installation
  6. Configuration
  7. Usage Examples
  8. Documentation Links
  9. Contributing Guidelines
  10. License
- **AND** each section SHALL use clear, descriptive headers
- **AND** the README SHALL use GitHub-Flavored Markdown formatting

#### Scenario: Code examples in README
- **WHEN** the README includes code examples
- **THEN** code blocks SHALL use appropriate syntax highlighting
- **AND** examples SHALL be accurate and runnable
- **AND** examples SHALL demonstrate common use cases
- **AND** complex examples SHALL include explanatory comments

### Requirement: README emphasizes architectural differentiators

The system SHALL ensure that OxenCode's unique architectural advantages are prominently featured and clearly explained in the README.

#### Scenario: Key features section
- **WHEN** a user reads the Key Features section
- **THEN** it SHALL highlight three core architectural differentiators:
  1. Agent/Tool Environment Separation - with explanation that agent execution environment is isolated from tool execution environment
  2. Long-Running Task Support - with explanation that OxenCode can handle extended tasks without interruption
  3. Innovative Context Management - with explanation that it uses multi-batch, user-transparent, asynchronous context compression vs. traditional full-context compression
- **AND** each feature SHALL include a concise, understandable description
- **AND** technical terms SHALL be explained or linked to detailed documentation

#### Scenario: Feature differentiation
- **WHEN** architectural features are described
- **THEN** the README SHALL explain how OxenCode differs from other AI coding assistants
- **AND** the README SHALL emphasize user benefits (not just technical details)
- **AND** the README SHALL avoid commodity features (e.g., basic tool support) unless they enable unique capabilities
