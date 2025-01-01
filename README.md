# LLM Terminal Chat

A terminal-based chat interface for LLMs.

## Configuration

The application requires configuration via environment variables. You can either set them in your environment or create a `.env` file in the project root.

Required environment variables:

- `LLM_ENDPOINT`: The URL of the LLM API endpoint
- `LLM_MODEL`: The model to use for chat

A `.env.example` file is provided as a template. To use it:

```bash
cp .env.example .env
# Edit .env with your preferred values
```

The application will display an error if any required environment variables are not set.
