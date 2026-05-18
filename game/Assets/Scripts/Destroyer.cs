using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class Destroyer : MonoBehaviour
{
    float time;

    public float min, max;



    private void Start()
    {
        time = Random.Range(min, max);
        Invoke("elo", time);
    }

    void elo()
    {
        Destroy(gameObject);
    }
}
