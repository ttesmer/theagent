source .env
curl -s https://openrouter.ai/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -d '{
  "model": "moonshotai/kimi-k2.5",
  "messages": [
    {
      "role": "user",
      "content": "Where are we?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "run_command",
        "description": "Run a shell command in the current location.",
        "parameters": 
          {
            "type": "object",
            "properties": 
              {
                "command": {"type": "string", "description": "The shell command to be run"},
              },
            "required": ["command"]
         }
       }
     }
   ],
}' | python3 -m json.tool
