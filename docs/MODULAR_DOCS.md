# GhostCLI Modular Documentation Guide

## Overview

The GhostCLI documentation has been reorganized into a modular structure to improve maintainability and navigation. The spec files (`.kiro/specs/ghostcli-proxy/`) remain intact for the Kiro spec system, while the new modular docs provide better organization.

## Documentation Structure

```
docs/
├── MODULAR_DOCS.md (this file)
├── modules/
│   ├── README.md (module overview and navigation)
│   ├── 01-http-server/
│   │   ├── README.md (module overview)
│   │   ├── requirements.md (extracted requirements)
│   │   └── design.md (detailed design)
│   ├── 02-translation-engine/
│   │   ├── README.md
│   │   ├── requirements.md
│   │   └── design.md
│   ├── 03-provider-adapters/
│   │   ├── README.md
│   │   ├── requirements.md
│   │   ├── design.md
│   │   └── patterns/
│   │       ├── openai-compatible.md
│   │       ├── anthropic-native.md
│   │       └── aws-eventstream.md
│   ├── 04-cli/
│   │   ├── README.md
│   │   ├── requirements.md
│   │   └── design.md
│   ├── 05-security/
│   │   ├── README.md
│   │   ├── requirements.md
│   │   └── design.md
│   └── 06-observability/
│       ├── README.md
│       ├── requirements.md
│       └── design.md
└── architecture/
    ├── overview.md (system-wide architecture)
    ├── data-flow.md (request/response flow)
    └── deployment.md (deployment guide)
```

## Module Organization

### Module Structure

Each module follows a consistent structure:

1. **README.md**: High-level overview
   - Module purpose and responsibilities
   - Architecture diagram
   - Related requirements (by number)
   - Key components summary
   - Links to detailed docs

2. **requirements.md**: Extracted requirements
   - Only requirements relevant to this module
   - Full acceptance criteria
   - User stories

3. **design.md**: Detailed technical design
   - Component architecture
   - Code examples
   - Data structures
   - Implementation details
   - Performance considerations
   - Error handling

## Modules

### 01. HTTP Server
**Focus**: HTTP server initialization, routing, middleware, health checks

**Files**:
- `01-http-server/README.md` - Overview
- `01-http-server/requirements.md` - Requirements 1, 13, 22, 28, 29
- `01-http-server/design.md` - Server implementation details

### 02. Translation Engine
**Focus**: Bidirectional translation between Anthropic and Unified Protocol

**Files**:
- `02-translation-engine/README.md` - Overview
- `02-translation-engine/requirements.md` - Requirements 2, 5, 14, 15, 24, 25
- `02-translation-engine/design.md` - Parser/formatter implementation

### 03. Provider Adapters
**Focus**: Provider integration using pattern-based architecture

**Files**:
- `03-provider-adapters/README.md` - Overview
- `03-provider-adapters/requirements.md` - Requirements 3, 4, 6, 7, 8, 19, 20, 27
- `03-provider-adapters/design.md` - Adapter implementation
- `03-provider-adapters/patterns/` - Pattern-specific docs

### 04. CLI
**Focus**: Command-line interface and user interaction

**Files**:
- `04-cli/README.md` - Overview
- `04-cli/requirements.md` - Requirements 9, 11, 12, 21, 26, 30
- `04-cli/design.md` - CLI implementation

### 05. Security
**Focus**: Secure API key storage and credential management

**Files**:
- `05-security/README.md` - Overview
- `05-security/requirements.md` - Requirement 10
- `05-security/design.md` - Security implementation

### 06. Observability
**Focus**: Logging, metrics, and monitoring

**Files**:
- `06-observability/README.md` - Overview
- `06-observability/requirements.md` - Requirements 16, 17
- `06-observability/design.md` - Logging/metrics implementation

## Navigation

### Starting Points

1. **New to GhostCLI?**
   - Start with `modules/README.md` for system overview
   - Read module READMEs for high-level understanding

2. **Implementing a Module?**
   - Read module README for context
   - Review requirements.md for acceptance criteria
   - Follow design.md for implementation details

3. **Adding a Provider?**
   - Read `03-provider-adapters/README.md`
   - Check pattern family in `03-provider-adapters/patterns/`
   - Follow adapter implementation guide

4. **Debugging an Issue?**
   - Check relevant module README
   - Review error handling in design.md
   - Check observability module for logging

## Relationship to Spec Files

### Spec Files (`.kiro/specs/ghostcli-proxy/`)
- **Purpose**: Used by Kiro spec execution system
- **Format**: Standardized (requirements.md, design.md, tasks.md)
- **Content**: Complete, monolithic documents
- **Status**: Keep as-is (required by spec system)

### Modular Docs (`docs/modules/`)
- **Purpose**: Developer reference and implementation guide
- **Format**: Modular, organized by feature
- **Content**: Extracted and reorganized from spec files
- **Status**: New, parallel documentation structure

### When to Use Which

**Use Spec Files When**:
- Running Kiro spec tasks
- Generating task lists
- Tracking spec progress
- Formal requirements review

**Use Modular Docs When**:
- Implementing a specific module
- Understanding system architecture
- Adding new features
- Debugging issues
- Onboarding new developers

## Benefits of Modular Structure

### For Developers
- **Focused Context**: Only read what's relevant to your work
- **Easier Navigation**: Find information by feature/module
- **Better Organization**: Related content grouped together
- **Reduced Cognitive Load**: Smaller, digestible documents

### For Maintainability
- **Isolated Changes**: Update one module without affecting others
- **Clear Ownership**: Each module has clear boundaries
- **Easier Reviews**: Review changes to specific modules
- **Better Testing**: Test modules independently

### For Onboarding
- **Progressive Learning**: Learn one module at a time
- **Clear Dependencies**: Understand module relationships
- **Practical Examples**: Module-specific code examples
- **Focused Documentation**: No need to read entire spec

## Next Steps

### Completed
✅ Created modular documentation structure
✅ Extracted requirements by module
✅ Created module READMEs with overviews
✅ Created design docs for HTTP Server module

### To Complete
- [ ] Complete design.md for remaining modules
- [ ] Create pattern-specific docs in provider-adapters
- [ ] Add architecture overview docs
- [ ] Add data flow diagrams
- [ ] Add deployment guide

### How to Contribute
1. Follow the module structure template
2. Keep READMEs concise (overview only)
3. Put details in design.md
4. Include code examples
5. Add diagrams where helpful
6. Link between related modules

## Questions?

- **Where do I find X?** Check `modules/README.md` for navigation
- **How do I add a module?** Follow the structure template above
- **Should I update spec files?** No, only update modular docs
- **What about tasks.md?** Tasks remain in spec files only
