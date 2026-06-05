using System;
using System.Collections;
using System.Collections.Generic;
using System.Text;
using TMPro;
using UnityEngine;
using UnityEngine.Networking;

//api sending classes
public class LoginUserDataSend
{
    public string login;
    public string password;
    public LoginUserDataSend(string _login, string _password)
    {
        login = _login;
        password = _password;
    }
}

public class UserDataSend
{
    public string token;
    public UserDataSend(string _token)
    {
        token = _token;
    }
}

//api response classes
public class ApiResponse
{
    public bool sukces;
    public string msg;
}


public class sseResponse
{
    public string ip;
    public int port;
}

public class URL
{
    public string loginUrl, registerUrl, waitingUrl, connectionUrl;
    public URL(string prefix)
    {
        loginUrl = prefix + "/api/login";
        registerUrl = prefix + "/api/register";
        waitingUrl = prefix + "/api/waiting";
        connectionUrl = prefix + "/api/connection";
    }
}

public class ApiSender : MonoBehaviour
{
    public static ApiSender Instance { get; set; }
    public URL urls = new URL("http://localhost:5173");

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
    public IEnumerator SendQuery(object data, string url, Action<string> callback, string meth = "POST")
    {
        string jsonDane = JsonUtility.ToJson(data);

        UnityWebRequest www = new UnityWebRequest(url, meth);

        byte[] bodyRaw = Encoding.UTF8.GetBytes(jsonDane);
        www.uploadHandler = new UploadHandlerRaw(bodyRaw);
        www.downloadHandler = new DownloadHandlerBuffer();
        www.SetRequestHeader("Content-Type", "application/json");
        yield return www.SendWebRequest();

        if (www.result != UnityWebRequest.Result.Success)
        {
            Debug.LogError("Coœ posz³o nie tak z po³¹czeniem siê z API");
            callback?.Invoke(null);
        }
        else
        {
            callback?.Invoke(www.downloadHandler.text);
        }
    }
}