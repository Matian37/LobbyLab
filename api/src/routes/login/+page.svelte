<script>
    import { goto } from "$app/navigation";

    let login = $state(''), password = $state(''), tekstBledu = $state("");
    
    function changePage(path){
        goto(path);
    }

    async function submit(){
        let data = {login: login, password: password};
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        });
        console.log("wrocilo mi info");

        const sukces = await response.json();
        if(sukces.sukces){
            console.log("poprawnie zalogowano");
            tekstBledu = "";
            changePage('/')
        }
        else{
            tekstBledu = sukces.msg;
        }
    }
</script>

<button onclick={()=>changePage('/')}>Powrót</button>
Login <input bind:value={login} data-testid="login-input"/>
Password <input bind:value={password} type="password" data-testid="password-input"/>
<button onclick={()=>submit()} data-testid="login-apply" >Submit</button>

<p data-testid="error-text">{tekstBledu}</p>

<style>
    /* Wyśrodkowanie i podstawowe style tła dla całej strony */
    :global(body) {
      text-align: center;
      background-color: #0f172a;
      color: #f8fafc;
      font-family: system-ui, -apple-system, sans-serif;
      margin: 2rem 0;
    }
  
    /* Przycisk Powrót — wymusza osobną linię na samej górze */
    button:first-of-type {
      display: block;
      margin: 0 auto 1.5rem auto;
    }
  
    /* Pola tekstowe w osobnych liniach */
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
  
    /* Podstawowe style dla przycisków */
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
  
    /* Przycisk Submit / Login */
    button[data-testid='login-apply'] {
      background-color: #2563eb;
      border-color: #3b82f6;
      font-weight: 600;
      width: 260px;
      display: block;
    }
  
    button[data-testid='login-apply']:hover {
      background-color: #1d4ed8;
    }
  
    /* Informacja o błędzie */
    p[data-testid="error-text"] {
      color: #ef4444;
      font-size: 0.9rem;
      margin-top: 1rem;
      min-height: 1.2em;
    }
  </style>