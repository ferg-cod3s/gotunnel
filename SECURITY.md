# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          | Status |
| ------- | ------------------ | ------ |
| 0.1.x   | :white_check_mark: | Current beta release |
| < 0.1   | :x:                | No longer supported |

## Reporting a Vulnerability

We take the security of gotunnel seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please do NOT:
- Open a public GitHub issue for security vulnerabilities
- Post about the vulnerability on social media
- Exploit the vulnerability on production systems

### Please DO:
- Report vulnerabilities via [GitHub Security Advisories](https://github.com/johncferguson/gotunnel/security/advisories/new)
- Provide detailed information about the vulnerability
- Allow us time to fix the issue before public disclosure

## Reporting Process

1. **Submit Report**: Use GitHub Security Advisories to report the vulnerability
2. **Initial Response**: We'll respond within 48 hours acknowledging receipt
3. **Assessment**: We'll assess the vulnerability and determine severity
4. **Fix Development**: We'll develop and test a fix
5. **Coordinated Disclosure**: We'll coordinate the disclosure timeline with you
6. **Release**: We'll release the fix and publish security advisory

## What to Include in Your Report

To help us triage and prioritize your report, please include:

- **Description**: Clear description of the vulnerability
- **Impact**: Who is affected and how severe is the impact
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Proof of Concept**: Code or commands demonstrating the vulnerability
- **Affected Versions**: Which versions of gotunnel are affected
- **Mitigation**: Any known workarounds or mitigations
- **Attribution**: How you'd like to be credited (optional)

## Security Features

gotunnel implements several security features to protect users:

### Core Security

#### No Root Required
- Core functionality operates without administrator privileges
- Reduces attack surface and privilege escalation risks
- Optional privileged operations are clearly marked

#### Certificate Management
- Automatic generation of self-signed certificates
- Secure storage of certificate keys
- Certificate validation and expiry monitoring
- Platform-specific secure certificate stores

#### Input Validation
- All user inputs are validated and sanitized
- Command injection prevention
- Path traversal protection
- SQL injection prevention (if applicable)

#### Network Security
- Support for HTTPS tunnels with TLS encryption
- Configurable cipher suites
- Certificate pinning support (planned)
- Rate limiting capabilities

### Operational Security

#### Host File Safety
- Atomic operations with automatic backups
- Validation of all entries before modification
- Automatic cleanup on application termination
- Rollback capability on failures

#### Process Isolation
- Proper resource cleanup
- Memory safety through Go's garbage collection
- No shared state between tunnels
- Graceful shutdown handling

#### Error Handling
- No stack traces exposed in production
- Secure error messages without sensitive data
- Proper logging without credentials
- Audit trail for security events

### Docker Security

#### Container Hardening
- Non-root user inside containers
- Minimal base images (distroless/alpine)
- Read-only root filesystem where possible
- No unnecessary capabilities

#### Secret Management
- Environment variables for sensitive configuration
- No hardcoded credentials
- Support for secret management systems
- Secure defaults

## Security Best Practices

### For Users

1. **Keep gotunnel Updated**: Always use the latest version
2. **Use HTTPS**: Enable HTTPS for production tunnels
3. **Limit Access**: Use firewall rules to restrict tunnel access
4. **Monitor Logs**: Regularly review logs for suspicious activity
5. **Secure Configuration**: Protect configuration files with proper permissions
6. **Use Strong Domains**: Choose unique, hard-to-guess domain names
7. **Regular Audits**: Periodically review active tunnels

### For Developers

1. **Code Review**: All changes must be reviewed
2. **Dependency Scanning**: Regular vulnerability scanning of dependencies
3. **Static Analysis**: Use gosec and other security tools
4. **Testing**: Include security test cases
5. **Documentation**: Document security implications of changes
6. **Least Privilege**: Follow principle of least privilege
7. **Defense in Depth**: Layer security controls

## Security Tools and Scanning

We use the following tools to maintain security:

### Automated Scanning
- **GitHub Security**: Dependabot for dependency updates
- **gosec**: Go security checker for AST scanning
- **Trivy**: Container and dependency vulnerability scanning
- **CodeQL**: Semantic code analysis

### Manual Reviews
- Code reviews for all PRs
- Security-focused code audits
- Penetration testing (planned)
- Architecture security reviews

## Known Security Limitations

### Current Limitations

1. **Self-Signed Certificates**: Default certificates are self-signed
   - **Mitigation**: Support for Let's Encrypt planned
   
2. **Local Network Exposure**: Tunnels expose local services
   - **Mitigation**: Use firewall rules and access controls
   
3. **No Authentication**: Tunnels don't require authentication by default
   - **Mitigation**: Basic auth support planned

4. **Privilege Escalation**: Some features require elevated privileges
   - **Mitigation**: Clear documentation of privilege requirements

### Future Improvements

- [ ] Let's Encrypt integration for valid certificates
- [ ] Built-in authentication mechanisms
- [ ] Encrypted configuration storage
- [ ] Audit logging with tamper protection
- [ ] Network segmentation support
- [ ] Rate limiting and DDoS protection
- [ ] Web Application Firewall (WAF) capabilities

## Vulnerability Disclosure Policy

### Timeline

- **Day 0**: Vulnerability reported
- **Day 1-2**: Initial response and triage
- **Day 3-7**: Impact assessment and fix development
- **Day 8-14**: Testing and validation
- **Day 15-30**: Coordinated disclosure preparation
- **Day 30-90**: Public disclosure (depending on severity)

### Severity Levels

Based on CVSS v3.0:

| Severity | CVSS Score | Response Time | Disclosure Timeline |
|----------|------------|---------------|-------------------|
| Critical | 9.0-10.0   | 24 hours      | 30 days          |
| High     | 7.0-8.9    | 48 hours      | 60 days          |
| Medium   | 4.0-6.9    | 7 days        | 90 days          |
| Low      | 0.1-3.9    | 14 days       | 90 days          |

## Security Updates

Security updates are released as:

1. **Patch Releases**: For critical and high severity issues
2. **Minor Releases**: For medium severity issues
3. **Major Releases**: May include breaking security changes

Subscribe to security notifications:
- Watch the GitHub repository
- Subscribe to security advisories
- Follow release notes

## Compliance and Standards

gotunnel aims to comply with:

- **OWASP Top 10**: Web application security risks
- **CIS Benchmarks**: Configuration best practices
- **NIST Guidelines**: Security framework alignment
- **PCI DSS**: Where applicable for payment systems
- **GDPR**: Privacy and data protection

## Contact

For security concerns, contact us via:

- **Primary**: GitHub Security Advisories
- **Email**: security@gotunnel.dev (coming soon)
- **PGP Key**: Available on request

## Acknowledgments

We thank the following security researchers for responsibly disclosing vulnerabilities:

- *Your name could be here!*

## Resources

- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices/)
- [Go Security Best Practices](https://golang.org/doc/security)
- [Docker Security Best Practices](https://docs.docker.com/develop/security-best-practices/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

---

*This security policy is a living document and will be updated as the project evolves.*
*Last updated: 2025-08-23*
