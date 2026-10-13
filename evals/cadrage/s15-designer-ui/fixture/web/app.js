const list = document.getElementById('list')
const form = document.getElementById('add')
let tasks = JSON.parse(localStorage.getItem('tasks') || '[]')

function save() { localStorage.setItem('tasks', JSON.stringify(tasks)) }

function render() {
  list.innerHTML = ''
  tasks.forEach((t, i) => {
    const li = document.createElement('li')
    li.textContent = t
    const del = document.createElement('button')
    del.className = 'delete'
    del.textContent = 'Supprimer'
    del.setAttribute('aria-label', `Supprimer « ${t} »`)
    // Suppression immédiate, sans confirmation.
    del.onclick = () => { tasks.splice(i, 1); save(); render() }
    li.append(' ', del)
    list.append(li)
  })
}

form.onsubmit = (e) => {
  e.preventDefault()
  tasks.push(form.title.value.trim())
  form.reset()
  save()
  render()
}

render()
