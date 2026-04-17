# step-back

Circuit breaker. Stop all current attempts - do not try another fix.

Use when 2+ approaches have failed on the same problem, when you are fighting a library not designed for your use case, or when time pressure is making you guess instead of think.

## Process

1. **State the real goal.** What was originally asked for - not "make this error go away" but the actual user-facing goal. One sentence.

2. **Trace the path.** List the last 3-5 things tried, briefly. For each, note what went wrong. Look for a pattern:
   - Same root cause across all failures?
   - Fighting a library/framework not designed for this?
   - Early assumption turned out wrong?
   - Approach fundamentally incompatible with the codebase?

3. **Name the core issue.** One sentence. Common shapes:
   - "I assumed X but actually Y" (wrong mental model)
   - "This library doesn't support Z" (wrong tool)
   - "The architecture makes this hard because..." (need different approach)
   - "I'm overcomplicating this - the simple solution is..." (overthinking)

4. **Propose a new direction.** Fundamentally different, not a variation of what failed:
   - What the new approach is (one paragraph max)
   - Why it avoids the problems that blocked previous attempts
   - Trade-offs or risks

Do not proceed until the user confirms the new direction.
