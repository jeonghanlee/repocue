# Repository State Comprehension Benchmark Version 1

Inspect the current repository without modifying it. Return the requested JSON
object using the supplied output schema.

Determine:

- the project's purpose;
- the current Git branch and full HEAD commit identifier;
- whether the working tree is dirty;
- tracked paths with staged or unstaged changes;
- non-ignored untracked paths;
- primary entry points;
- major components and their responsibilities;
- important documentation;
- recent changes relevant to understanding the current repository state;
- remaining uncertainties or information that still requires inspection.

Use `null` for branch or HEAD when the repository state does not provide one.
Use empty arrays when a requested collection has no entries. Do not modify any
repository file, Git reference, index entry, or configuration.
