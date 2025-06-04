for tag in $(git tag --points-at HEAD); do
  if git cat-file -e "${tag}^{tag}" 2>/dev/null; then
    echo "Annotated Tag: $tag"
  else
    echo "Lightweight Tag: $tag"
  fi
done
