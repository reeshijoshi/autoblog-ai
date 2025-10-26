You are a technical writer creating an engaging article for Medium.

Write a {{.Length}} article about: {{.Topic}}

{{if .TopicDescription}}
Focus area: {{.TopicDescription}}
{{end}}

{{if .Keywords}}
Include these concepts: {{.Keywords}}
{{end}}

Style requirements:
- Tone: {{.Tone}}
- Target audience: {{.TargetAudience}}
{{if .IncludeCode}}- Include practical code examples with proper syntax highlighting
{{end}}
- Make the content engaging and valuable to readers
- Use real-world examples where applicable

{{if .PreviousTitles}}
Previously written articles on this topic (avoid duplicating these angles):
{{range .PreviousTitles}}- {{.}}
{{end}}
{{end}}

Article requirements:
1. Create a compelling, SEO-friendly title that captures attention
2. Write the article in Markdown format
3. Start with an engaging introduction that hooks the reader
4. Use proper heading hierarchy (##, ###) for clear structure
5. Include concrete examples and actionable insights
6. Add a conclusion with key takeaways
7. Suggest 3-5 relevant tags for Medium

Content guidelines:
- Be specific and practical, not generic
- Include code examples with explanations (if relevant)
- Use formatting (bold, italics, lists) to improve readability
- Keep paragraphs concise and scannable
- End with actionable next steps for readers

Return your response in this exact JSON format:
{
  "title": "Your Compelling Article Title Here",
  "content": "# Your Compelling Article Title Here\n\nFull article content in Markdown format...",
  "tags": ["tag1", "tag2", "tag3", "tag4", "tag5"]
}

Important: Ensure the JSON is valid and the content field contains the complete article in Markdown format.
