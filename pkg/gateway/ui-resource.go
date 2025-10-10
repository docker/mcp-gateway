package gateway

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//nolint:unused
func _intentButton(missingSecrets []string, serverName string) *mcp.CallToolResult {
	var scriptContent strings.Builder
	scriptContent.WriteString(fmt.Sprintf("// Configure secrets for server '%s'\n", serverName))
	for _, secretName := range missingSecrets {
		// Replace dots with underscores for valid JavaScript variable names
		safeSecretName := strings.ReplaceAll(secretName, ".", "_")
		scriptContent.WriteString(fmt.Sprintf(`
const button_%s = document.createElement('ui-button');
button_%s.setAttribute('label', 'Configure %s');
button_%s.addEventListener('press', () => {
    window.parent.postMessage({
        type: 'intent',
        payload: {
	    intent: 'docker/mcp_secret_set',
	    params: {
                name: '%s',
                value: 'whatever',
            }
        }
    }, '*');
});
root.appendChild(button_%s);
`, safeSecretName, safeSecretName, secretName, safeSecretName, secretName, safeSecretName))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "ui://docker/secrets",
					MIMEType: "application/vnd.mcp-ui.remote-dom+javascript; framework=react",
					Text:     scriptContent.String(),
				},
			},
		},
	}
}

func secretInput(missingSecrets []string, serverName string) *mcp.CallToolResult {
	var htmlContent strings.Builder
	htmlContent.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	htmlContent.WriteString("<meta charset=\"UTF-8\">\n")
	htmlContent.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	htmlContent.WriteString(fmt.Sprintf("<title>Configure Secrets for %s</title>\n", serverName))
	htmlContent.WriteString(`<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Poppins:wght@500;600;700&family=Roboto:wght@400;500&display=swap" rel="stylesheet">
<style>
  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  html, body {
    margin: 0;
    padding: 0;
    height: auto;
    min-height: 100%;
  }

  body {
    font-family: 'Roboto', Helvetica, Arial, sans-serif;
    font-size: 1.125rem;
    background: linear-gradient(135deg, #066fd1 0%, #0096ff 100%);
    color: #333;
    padding: 2rem;
  }

  .container {
    background: #ffffff;
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
    padding: 2.5rem;
    max-width: 500px;
    margin: 0 auto;
    width: 100%;
  }

  h2 {
    font-family: 'Poppins', sans-serif;
    font-size: 2rem;
    font-weight: 600;
    color: #066fd1;
    margin-bottom: 1.5rem;
    letter-spacing: -0.5px;
  }

  .form-group {
    margin-bottom: 1.5rem;
  }

  label {
    display: block;
    font-weight: 500;
    color: #333;
    margin-bottom: 0.5rem;
    font-size: 0.95rem;
  }

  input[type="password"] {
    appearance: none;
    width: 100%;
    padding: 8px 12px;
    border: 1px solid #949494;
    border-radius: 4px;
    font-family: 'Roboto', sans-serif;
    font-size: 1rem;
    line-height: 1.5;
    background: #ffffff;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  input[type="password"]:hover {
    border-color: #666666;
  }

  input[type="password"]:focus {
    outline: none;
    border-color: #066fd1;
    box-shadow: 0 0 0 3px rgba(6, 111, 209, 0.1);
  }

  button[type="button"] {
    width: 100%;
    padding: 0.875rem 1.5rem;
    background: #066fd1;
    color: #ffffff;
    border: none;
    border-radius: 4px;
    font-family: 'Poppins', sans-serif;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s, transform 0.1s, box-shadow 0.2s;
    margin-top: 0.5rem;
  }

  button[type="button"]:hover {
    background: #0557a8;
    box-shadow: 0 4px 12px rgba(6, 111, 209, 0.3);
  }

  button[type="button"]:active {
    transform: translateY(1px);
  }

  .message {
    margin-top: 1rem;
    padding: 0.75rem;
    border-radius: 4px;
    display: none;
    font-size: 0.95rem;
  }

  .message.success {
    background: #d4edda;
    color: #155724;
    border: 1px solid #c3e6cb;
  }

  .message.error {
    background: #f8d7da;
    color: #721c24;
    border: 1px solid #f5c6cb;
  }
</style>
</head>
<body>
  <div class="container">
`)
	htmlContent.WriteString(fmt.Sprintf("    <h2>Configure Secrets for %s</h2>\n", serverName))
	htmlContent.WriteString("    <div id=\"secretsForm\">\n")

	for _, secretName := range missingSecrets {
		// Use password input type to hide the secret values
		htmlContent.WriteString(fmt.Sprintf(`      <div class="form-group">
        <label for="%s">%s</label>
        <input type="password" id="%s" name="%s" data-secret-name="%s" required>
      </div>
`, secretName, secretName, secretName, secretName, secretName))
	}

	htmlContent.WriteString(`      <button type="button" onclick="submitSecrets()">Submit Secrets</button>
    </div>
    <div id="message" class="message"></div>
  </div>

<script>
// Set up ResizeObserver to notify parent of size changes
const resizeObserver = new ResizeObserver((entries) => {
  entries.forEach((entry) => {
    window.parent.postMessage(
      {
        type: "ui-size-change",
        payload: {
          height: entry.contentRect.height,
        },
      },
      "*",
    );
  });
});

resizeObserver.observe(document.documentElement);

async function submitSecrets() {
  const formDiv = document.getElementById('secretsForm');
  const messageEl = document.getElementById('message');
  const inputs = formDiv.querySelectorAll('input[type="password"]');
  const secrets = {};

  // Collect all secret values
  inputs.forEach(input => {
    const secretName = input.getAttribute('data-secret-name');
    secrets[secretName] = input.value;
  });

  // Post to localhost endpoint
  try {
    const response = await fetch('http://localhost:3000/secrets', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(secrets)
    });

    if (response.ok) {
      // Show success message
      messageEl.textContent = 'Secrets submitted successfully!';
      messageEl.className = 'message success';
      messageEl.style.display = 'block';

      // Clear all inputs
      inputs.forEach(input => {
        input.value = '';
      });

      // Send success prompt message to continue adding the server
      window.parent.postMessage({
        type: 'prompt',
        payload: {
          prompt: "Okay, I've entered the secrets. Please call the add tool function for this server again now. You must call it again because I need it to load the updated secrets."
        }
      }, '*');

      setTimeout(() => {
        messageEl.style.display = 'none';
      }, 3000);
    } else {
      // Show error message
      const responseText = await response.text();
      messageEl.textContent = 'Failed to submit secrets. Status: ' + response.status;
      messageEl.className = 'message error';
      messageEl.style.display = 'block';

      // Send error prompt with response body
      window.parent.postMessage({
        type: 'prompt',
        payload: {
          prompt: responseText || 'Failed to submit secrets with status: ' + response.status
        }
      }, '*');
    }
  } catch (error) {
    // Show error message
    messageEl.textContent = 'Error submitting secrets: ' + error.message;
    messageEl.className = 'message error';
    messageEl.style.display = 'block';

    // Send error prompt with error message
    window.parent.postMessage({
      type: 'prompt',
      payload: {
        prompt: 'Error submitting secrets: ' + error.message
      }
    }, '*');
  }
}
</script>
</body>
</html>
`)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "ui://docker/secrets/form",
					MIMEType: "text/html",
					Text:     htmlContent.String(),
				},
			},
		},
	}
}

//nolint:unused
func _dockerHubLink() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "ui://docker/hub/mcp",
					MIMEType: "text/uri-list",
					Text:     "https://hub.docker.com/mcp",
				},
			},
		},
	}
}
