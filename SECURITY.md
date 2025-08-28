# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

The Mini-RAG team takes security bugs seriously. We appreciate your efforts to responsibly disclose your findings, and will make every effort to acknowledge your contributions.

### How to Report a Security Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them by email to: [your-email@domain.com]

Please include the following information in your report:

- Type of issue (e.g., buffer overflow, SQL injection, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- We will acknowledge receipt of your vulnerability report within 48 hours.
- We will provide a more detailed response within 72 hours indicating the next steps.
- We will keep you informed of the progress towards a fix and announcement.
- We may ask for additional information or guidance.

### Security Update Process

1. **Confirmation**: We will confirm the vulnerability and determine its impact.
2. **Fix Development**: We will develop a fix and create a security patch.
3. **Testing**: The fix will be thoroughly tested to ensure it resolves the issue without introducing new problems.
4. **Release**: We will release a new version with the security fix.
5. **Disclosure**: We will publish a security advisory with details about the vulnerability and the fix.

## Security Considerations

### Data Handling
- Mini-RAG processes and stores document content locally
- Vector embeddings are generated using Ollama (local or remote)
- All data is stored in local SQLite database files
- File paths may be stored in metadata

### Network Communication
- HTTP API server binds to configured host/port
- Ollama communication over HTTP
- No external service dependencies beyond Ollama

### Input Validation
- Document IDs and content are validated
- File uploads have size limits
- API endpoints validate request formats

### Dependencies
- Keep Go version updated for security patches
- Monitor sqlite-vec extension for updates
- Review Ollama security recommendations

## Best Practices for Users

### Secure Deployment
- Use HTTPS in production (reverse proxy recommended)
- Restrict network access to necessary clients
- Configure appropriate firewall rules
- Use strong authentication for multi-user environments

### Data Protection
- Store database files in secure locations
- Implement appropriate backup encryption
- Consider data retention policies
- Be mindful of sensitive content in documents

### Configuration Security
- Protect configuration files with sensitive settings
- Use environment variables for secrets when possible
- Regularly review and rotate any API keys or credentials
- Monitor access logs for suspicious activity

## Known Security Considerations

### CGO Dependencies
Mini-RAG uses CGO for SQLite and sqlite-vec integration. Keep the following in mind:

- Ensure your system's SQLite is up to date
- Review sqlite-vec extension security practices
- Consider the implications of native code execution

### File Processing
When processing uploaded files:

- Files are temporarily stored during processing
- PDF parsing involves native libraries
- Large files may consume significant memory
- File paths are stored in database metadata

## Reporting Other Security Issues

For questions about this security policy or general security practices, please contact the maintainers through:

- GitHub Issues (for non-sensitive discussions)
- Email (for sensitive matters)
- GitHub Discussions (for community input)

## Attribution

We believe in responsible disclosure and will give appropriate credit to security researchers who report vulnerabilities to us in a responsible manner.