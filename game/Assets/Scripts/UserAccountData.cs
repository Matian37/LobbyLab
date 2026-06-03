using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class UserAccountData : MonoBehaviour
{
    public static UserAccountData Instance { get; private set; }
    public string login;

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }
    private void Awake()
    {
        if (Instance is null)
        {
            Instance = this;
        }
        else
        {
            Destroy(gameObject);
        }

        DontDestroyOnLoad(gameObject);
    }

    public void SetLogin(string _login)
    {
        login = _login;
    }
}

