<script>
    import { goto } from "$app/navigation";

    let login = $state(''), password = $state(''), errorText = $state('');
    function changePage(path){
        goto(path);
    }

    async function submit(){
        if(password.length < 3 || password.length > 64)
        {
            errorText = 'Haslo musi miec co najmniej 3 znaki i co najwyżej 64';
            return;
        }
        let data = {login: login, password: password};
        let response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if(!result.sukces){
            errorText = result.msg;
        }
        else{
            console.debug("poprawnie zarejestrowano");
            changePage('/');
        }
    }
</script>

<button onclick={()=>changePage('/')}>Powrót</button>
Login <input bind:value={login} data-testid="login-input"/>
Password <input bind:value={password} type="password" data-testid="password-input"/>
<button onclick={()=>submit()} data-testid='register-apply'> Submit </button>
<p data-testid="error-text">{errorText}</p>


<style>
    button:first-of-type {
        display: block;
        margin: 0 auto 1.5rem auto;
    }

    /* Zamiast zmieniać całe body, wyśrodkowujemy same elementy na stronie */
    :global(body) {
      text-align: center;
      background-color: #0f172a;
      color: #f8fafc;
      font-family: system-ui, -apple-system, sans-serif;
      margin: 2rem 0;
    }
  
    /* Wymuszamy, aby napisy "Login" i "Password" były w osobnych liniach */
    input {
      display: block;
      width: 260px;
      margin: 0.5rem auto 1.25rem auto;
      padding: 0.65rem 0.85rem;
      font-size: 0.95rem;
      border-radius: 8px;
      border: 1px solid #334155;
      background-color: #1e293b;
      color: #f8fafc;
      box-sizing: border-box;
      outline: none;
      transition: border-color 0.2s ease;
    }
  
    input:focus {
      border-color: #38bdf8;
    }
  
    /* Przyciski ułożone ładnie pod sobą / obok siebie */
    button {
      display: inline-block;
      padding: 0.65rem 1.4rem;
      font-size: 0.95rem;
      font-weight: 500;
      color: #f8fafc;
      background-color: #334155;
      border: 1px solid #475569;
      border-radius: 8px;
      cursor: pointer;
      margin: 0.5rem auto;
      transition: all 0.2s ease;
    }
  
    button:hover {
      background-color: #475569;
    }
  
    /* Przycisk Submit */
    button[data-testid='register-apply'] {
      background-color: #2563eb;
      border-color: #3b82f6;
      font-weight: 600;
      width: 260px;
      display: block;
    }
  
    button[data-testid='register-apply']:hover {
      background-color: #1d4ed8;
    }
  
    /* Tekst błędu */
    p[data-testid="error-text"] {
      color: #ef4444;
      font-size: 0.9rem;
      margin-top: 1rem;
    }
</style>