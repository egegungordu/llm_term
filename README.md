# LLM Terminal Chat

A terminal-based chat interface for LLMs.

## Configuration

The application can be configured using environment variables. You can either set them in your environment or create a `.env` file in the project root.

Available environment variables:

- `LLM_ENDPOINT`: The URL of the LLM API endpoint (default: `http://167.235.207.146:11434/api/chat`)
- `LLM_MODEL`: The model to use for chat (default: `llama3.2`)

A `.env.example` file is provided as a template. To use it:

```bash
cp .env.example .env
# Edit .env with your preferred values
```
