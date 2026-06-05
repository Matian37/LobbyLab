using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class UserAccountData : MonoBehaviour
{
    public static UserAccountData Instance { get; private set; }
    public string login;
    public string token;
    public bool isLoggedIn = false;

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

    public void SetUser(string _login, string _token)
    {
        login = _login;
        token = _token;
        isLoggedIn = true;
        PlayerPrefs.SetString("sessionToken", _token);
        PlayerPrefs.SetString("sessionLogin", _login);
    }


    public void LogOut()
    {
        isLoggedIn = false;
        login = token = "";
        PlayerPrefs.DeleteKey("sessionToken");
        PlayerPrefs.DeleteKey("sessionLogin");
    }

    public void LoadUser()
    {
        if (PlayerPrefs.HasKey("sessionToken"))
        {
            isLoggedIn = true;
            token = PlayerPrefs.GetString("sessionToken");
            login = PlayerPrefs.GetString("sessionLogin");
        }

    }
}

